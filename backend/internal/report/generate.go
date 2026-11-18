package report

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Window is the report period in days.
const Window = 7

// Report is a finished weekly report.
type Report struct {
	Content  *Content
	Snapshot *Snapshot
	Email    *Email
	Usage    Usage
	Model    string
}

// Generator produces one report: Claude investigates through MCP tools and
// writes the words, then Go renders them next to numbers computed from data.
type Generator struct {
	Agent *Agent
}

// Generate runs a full report for the week ending now.
func (g *Generator) Generate(ctx context.Context) (*Report, error) {
	tools := g.Agent.Tools
	d, err := tools.Data(ctx)
	if err != nil {
		return nil, fmt.Errorf("load rank data: %w", err)
	}
	now := time.Now()
	if tools.Now != nil {
		now = tools.Now()
	}
	snap, err := d.Snapshot(now, Window)
	if err != nil {
		return nil, err
	}
	if snap.Me.Keywords == 0 {
		return nil, errors.New("no rank history yet for your app; run a rank check first")
	}

	raw, usage, err := g.Agent.Write(ctx, brief(snap))
	if err != nil {
		return nil, err
	}
	c, err := ParseContent(raw)
	if err != nil {
		return nil, err
	}
	if err := c.Normalize(func(id string) bool { _, ok := snap.Facts[id]; return ok }); err != nil {
		return nil, err
	}

	email, err := Render(c, snap, Meta{IconURL: g.iconURL(ctx, d)})
	if err != nil {
		return nil, fmt.Errorf("render email: %w", err)
	}
	model := g.Agent.Model
	if model == "" {
		model = DefaultModel
	}
	return &Report{Content: c, Snapshot: snap, Email: email, Usage: usage, Model: model}, nil
}

// iconURL fetches your app's icon for the masthead. The icon is decoration, so
// any failure just means the email goes out without it.
func (g *Generator) iconURL(ctx context.Context, d *Dataset) string {
	t := g.Agent.Tools
	if t.Listings == nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	listings, err := t.Listings(ctx, mostCommonCountry(d), d.Apps)
	if err != nil {
		return ""
	}
	for _, l := range listings {
		if l.IsMine && strings.HasPrefix(l.ArtworkURL, "https://") {
			return l.ArtworkURL
		}
	}
	return ""
}

func brief(s *Snapshot) string {
	var rivals []string
	for _, r := range s.Rivals {
		rivals = append(rivals, r.App)
	}
	comp := "no competitors tracked"
	if len(rivals) > 0 {
		comp = strings.Join(rivals, ", ")
	}
	return fmt.Sprintf(`Write this week's report.

Today is %s. The reporting window is the %d days ending now.
The reader's app is %q. It is tracked on %d keywords. Competitors: %s.

Investigate with the tools, then answer with the report JSON.`,
		s.AsOf.Format("Monday, 2 January 2006"), s.Days, s.Mine.Name, s.Me.Keywords, comp)
}

// CacheListings memoises a listings lookup so the masthead icon and the
// agent's get_app_listings call share one request to Apple.
func CacheListings(fn func(context.Context, string, []App) ([]Listing, error), ttl time.Duration) func(context.Context, string, []App) ([]Listing, error) {
	var (
		mu     sync.Mutex
		cached []Listing
		at     time.Time
		have   bool
	)
	return func(ctx context.Context, country string, apps []App) ([]Listing, error) {
		mu.Lock()
		defer mu.Unlock()
		if have && time.Since(at) < ttl {
			return cached, nil
		}
		out, err := fn(ctx, country, apps)
		if err != nil {
			return nil, err
		}
		cached, at, have = out, time.Now(), true
		return out, nil
	}
}
