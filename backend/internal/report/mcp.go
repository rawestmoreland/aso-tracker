package report

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"aso-tracker/internal/itunes"
)

// Listing is an app's public App Store listing, for ASO context (titles,
// ratings, descriptions) that rank history alone can't give.
type Listing struct {
	Name        string  `json:"name"`
	IsMine      bool    `json:"is_mine"`
	Seller      string  `json:"seller"`
	Genre       string  `json:"genre"`
	Rating      float64 `json:"average_rating"`
	RatingCount int     `json:"rating_count"`
	Version     string  `json:"version"`
	LastUpdate  string  `json:"last_updated"`
	Description string  `json:"description_excerpt"`
	ReleaseNote string  `json:"release_notes_excerpt"`
	ArtworkURL  string  `json:"artwork_url"`
}

// Previous is last week's report, so this week's can follow up on it.
type Previous struct {
	SentAt   time.Time `json:"sent_at"`
	Subject  string    `json:"subject"`
	Headline string    `json:"headline"`
	Intro    string    `json:"intro"`
	Quests   []string  `json:"recommended_quests"`
}

// Tools backs the MCP tools. Every tool is read-only. Optional funcs may be nil.
type Tools struct {
	Data     func(ctx context.Context) (*Dataset, error)
	Listings func(ctx context.Context, country string, apps []App) ([]Listing, error)
	Previous func(ctx context.Context) (*Previous, error)
	Now      func() time.Time
}

// CacheDataset memoises a loader so one report run (or a burst of chat
// questions) reads the database once.
func CacheDataset(load func(ctx context.Context) (*Dataset, error), ttl time.Duration) func(context.Context) (*Dataset, error) {
	var (
		mu     sync.Mutex
		cached *Dataset
		at     time.Time
	)
	return func(ctx context.Context) (*Dataset, error) {
		mu.Lock()
		defer mu.Unlock()
		if cached != nil && time.Since(at) < ttl {
			return cached, nil
		}
		d, err := load(ctx)
		if err != nil {
			return nil, err
		}
		cached, at = d, time.Now()
		return d, nil
	}
}

// ITunesListings adapts the iTunes lookup endpoint to Tools.Listings.
func ITunesListings(c *itunes.Client) func(context.Context, string, []App) ([]Listing, error) {
	return func(ctx context.Context, country string, apps []App) ([]Listing, error) {
		ids := make([]int64, 0, len(apps))
		byID := map[int64]App{}
		for _, a := range apps {
			if a.TrackID != 0 {
				ids = append(ids, a.TrackID)
				byID[a.TrackID] = a
			}
		}
		if len(ids) == 0 {
			return nil, nil
		}
		found, err := c.Lookup(ctx, country, ids...)
		if err != nil {
			return nil, err
		}
		var out []Listing
		for _, f := range found {
			a := byID[f.TrackID]
			out = append(out, Listing{
				Name: f.TrackName, IsMine: a.IsMine, Seller: f.SellerName, Genre: f.PrimaryGenre,
				Rating: round1(f.AverageRating), RatingCount: f.RatingCount, Version: f.Version,
				LastUpdate: f.CurrentVersionDate, Description: clip(f.Description, 700),
				ReleaseNote: clip(f.ReleaseNotes, 300), ArtworkURL: f.Artwork,
			})
		}
		sort.SliceStable(out, func(i, j int) bool { return out[i].IsMine && !out[j].IsMine })
		return out, nil
	}
}

// --- tool inputs and outputs -------------------------------------------------

type windowIn struct {
	Days int `json:"days,omitempty" jsonschema:"look-back window in days (1-30, default 7)"`
}

type limitIn struct {
	Days  int `json:"days,omitempty" jsonschema:"look-back window in days (1-30, default 7)"`
	Limit int `json:"limit,omitempty" jsonschema:"max items per list (1-25, default 8)"`
}

type overviewOut struct {
	AsOf        string     `json:"as_of"`
	WindowDays  int        `json:"window_days"`
	MyApp       Overview   `json:"my_app"`
	Competitors []Overview `json:"competitors"`
	Notes       string     `json:"notes"`
}

type moversOut struct {
	Up   []KeywordFact `json:"biggest_gains"`
	Down []KeywordFact `json:"biggest_losses"`
}

type rivalsOut struct {
	Competitors []RivalReport `json:"competitors"`
}

type historyIn struct {
	Keyword string `json:"keyword" jsonschema:"keyword id (as returned by other tools) or exact search term"`
	Days    int    `json:"days,omitempty" jsonschema:"days of history (1-30, default 14)"`
}

type historyOut struct {
	Keyword Keyword      `json:"keyword"`
	Apps    []AppHistory `json:"apps"`
	Note    string       `json:"note"`
}

type findIn struct {
	Query string `json:"query" jsonschema:"case-insensitive substring of the search term"`
	Days  int    `json:"days,omitempty" jsonschema:"look-back window in days (1-30, default 7)"`
	Limit int    `json:"limit,omitempty" jsonschema:"max results (1-50, default 15)"`
}

type findOut struct {
	Matches []KeywordFact `json:"matches"`
}

type listingsOut struct {
	Apps []Listing `json:"apps"`
	Note string    `json:"note"`
}

type previousOut struct {
	Available bool      `json:"available"`
	Report    *Previous `json:"report,omitempty"`
}

type empty struct{}

func clamp(v, def, lo, hi int) int {
	if v == 0 {
		return def
	}
	return min(max(v, lo), hi)
}

const rankNote = "Rank 0 means not in the App Store's top 200 for that search. " +
	"change is positions gained (+) or lost (-); unranked counts as position 201. " +
	"daily_ranks run oldest to newest with -1 where the keyword wasn't tracked yet. " +
	"visibility is the mean over keywords of (101 - rank), 0 outside the top 100, so 0-100."

// Server builds the MCP server. It is used in-process by the report agent and
// can also be mounted over HTTP for ad-hoc questions from Claude Desktop/Code.
func (t *Tools) Server() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "aso-tracker", Version: "1.0.0"}, nil)
	ro := &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: ptr(false)}

	// load resolves the data and the reference time for one call.
	load := func(ctx context.Context) (*Dataset, time.Time, error) {
		d, err := t.Data(ctx)
		if err != nil {
			return nil, time.Time{}, err
		}
		now := time.Now()
		if t.Now != nil {
			now = t.Now()
		}
		return d, now, nil
	}

	mcp.AddTool(s, &mcp.Tool{
		Name: "get_overview",
		Description: "Scoreboard for your app and every competitor over a window: visibility score (0-100) and its change, " +
			"how many keywords sit in the top 3 / top 10 / top 50, average rank, and a daily visibility trend. Start here.",
		Annotations: ro,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in windowIn) (*mcp.CallToolResult, overviewOut, error) {
		d, now, err := load(ctx)
		if err != nil {
			return nil, overviewOut{}, err
		}
		snap, err := d.Snapshot(now, clamp(in.Days, 7, 1, 30))
		if err != nil {
			return nil, overviewOut{}, err
		}
		return nil, overviewOut{AsOf: now.Format(time.RFC3339), WindowDays: snap.Days, MyApp: snap.Me, Competitors: snap.Rivals, Notes: rankNote}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "get_movers",
		Description: "Your app's biggest keyword gains and losses over the window, largest first. " +
			"Includes each keyword's id, rank now vs before, the best-ranked competitor, and daily ranks.",
		Annotations: ro,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in limitIn) (*mcp.CallToolResult, moversOut, error) {
		d, now, err := load(ctx)
		if err != nil {
			return nil, moversOut{}, err
		}
		mine, err := d.Mine()
		if err != nil {
			return nil, moversOut{}, err
		}
		facts := d.Facts(mine, now, clamp(in.Days, 7, 1, 30))
		n := clamp(in.Limit, 8, 1, 25)
		return nil, moversOut{Up: Movers(facts, "up", n), Down: Movers(facts, "down", n)}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "get_opportunities",
		Description: "Keywords most worth working on: striking_distance (you rank 11-30), podium_push (you rank 4-10), " +
			"and competitor_gaps (a competitor is in the top 10 and you rank lower or not at all, biggest gap first).",
		Annotations: ro,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in limitIn) (*mcp.CallToolResult, Opportunities, error) {
		d, now, err := load(ctx)
		if err != nil {
			return nil, Opportunities{}, err
		}
		mine, err := d.Mine()
		if err != nil {
			return nil, Opportunities{}, err
		}
		return nil, FindOpportunities(d.Facts(mine, now, clamp(in.Days, 7, 1, 30)), clamp(in.Limit, 8, 1, 25)), nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "get_competitor_moves",
		Description: "What each competitor did over the window: their scoreboard plus the keywords where they " +
			"climbed or slipped the most.",
		Annotations: ro,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in limitIn) (*mcp.CallToolResult, rivalsOut, error) {
		d, now, err := load(ctx)
		if err != nil {
			return nil, rivalsOut{}, err
		}
		return nil, rivalsOut{Competitors: d.RivalReports(now, clamp(in.Days, 7, 1, 30), clamp(in.Limit, 4, 1, 10))}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_keyword_history",
		Description: "Daily ranks for one keyword across your app and every competitor. Use it to see how a move unfolded.",
		Annotations: ro,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in historyIn) (*mcp.CallToolResult, historyOut, error) {
		d, now, err := load(ctx)
		if err != nil {
			return nil, historyOut{}, err
		}
		kw, ok := d.Keywords[in.Keyword]
		if !ok {
			for _, k := range d.Keywords {
				if strings.EqualFold(k.Term, strings.TrimSpace(in.Keyword)) {
					kw, ok = k, true
					break
				}
			}
		}
		if !ok {
			return nil, historyOut{}, fmt.Errorf("unknown keyword %q", in.Keyword)
		}
		return nil, historyOut{Keyword: kw, Apps: d.History(kw.ID, now, clamp(in.Days, 14, 1, 30)), Note: rankNote}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "find_keywords",
		Description: "Look up how your app ranks on tracked keywords whose term contains some text, e.g. \"habit\".",
		Annotations: ro,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in findIn) (*mcp.CallToolResult, findOut, error) {
		d, now, err := load(ctx)
		if err != nil {
			return nil, findOut{}, err
		}
		mine, err := d.Mine()
		if err != nil {
			return nil, findOut{}, err
		}
		q := strings.ToLower(strings.TrimSpace(in.Query))
		var out findOut
		for _, f := range d.Facts(mine, now, clamp(in.Days, 7, 1, 30)) {
			if strings.Contains(strings.ToLower(f.Term), q) {
				out.Matches = append(out.Matches, f)
			}
		}
		out.Matches = capN(out.Matches, clamp(in.Limit, 15, 1, 50))
		return nil, out, nil
	})

	if t.Listings != nil {
		mcp.AddTool(s, &mcp.Tool{
			Name: "get_app_listings",
			Description: "Public App Store listing for your app and each competitor: name, genre, rating and review count, " +
				"last update, and description/release-notes excerpts. Use it to compare titles and positioning when " +
				"recommending metadata changes. Live data from Apple; call it at most once.",
			Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		}, func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, listingsOut, error) {
			d, _, err := load(ctx)
			if err != nil {
				return nil, listingsOut{}, err
			}
			apps, err := t.Listings(ctx, mostCommonCountry(d), d.Apps)
			if err != nil {
				return nil, listingsOut{}, err
			}
			return nil, listingsOut{Apps: apps, Note: "Subtitle and keyword field are not public; infer from name and description."}, nil
		})
	}

	if t.Previous != nil {
		mcp.AddTool(s, &mcp.Tool{
			Name: "get_previous_report",
			Description: "Last week's report (headline, intro and the quests it recommended) so you can follow up: " +
				"did the advice land, did the rank move?",
			Annotations: ro,
		}, func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, previousOut, error) {
			p, err := t.Previous(ctx)
			if err != nil {
				return nil, previousOut{}, err
			}
			if p == nil {
				return nil, previousOut{Available: false}, nil
			}
			return nil, previousOut{Available: true, Report: p}, nil
		})
	}

	return s
}

func ptr[T any](v T) *T { return &v }

func mostCommonCountry(d *Dataset) string {
	counts := map[string]int{}
	for _, k := range d.Keywords {
		counts[k.Country]++
	}
	best, bestN := "us", 0
	for c, n := range counts {
		if n > bestN || (n == bestN && c < best) {
			best, bestN = c, n
		}
	}
	return best
}
