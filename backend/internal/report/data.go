// Package report turns rank history into a weekly email.
//
// The work is split so that the model only ever supplies words:
//
//   - data.go / analysis.go: deterministic numbers computed from rank_history.
//   - mcp.go: those numbers exposed as read-only MCP tools.
//   - agent.go: Claude explores the tools and returns a structured Content.
//   - render.go: Content + numbers become the HTML/text email.
//
// Every figure in the email is looked up from the data by keyword ID, so a
// model slip can never put a wrong rank in front of the reader.
package report

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// Unranked is the effective position of an app that is not in the top 200
// (rank 0 in the database). It matches the Gaps page in the frontend.
const Unranked = 201

// HistoryDays is how much history Load pulls: enough for a 30 day lookback
// plus the daily sparkline window.
const HistoryDays = 35

// App is a tracked app (yours or a competitor).
type App struct {
	ID      string `json:"-"`
	Name    string `json:"name"`
	TrackID int64  `json:"track_id"`
	IsMine  bool   `json:"is_mine"`
}

// Keyword is a tracked search term in one storefront.
type Keyword struct {
	ID      string `json:"id"`
	Term    string `json:"term"`
	Country string `json:"country"`
}

// Point is one rank observation. Rank 0 means "not in the top 200".
type Point struct {
	At   time.Time
	Rank int
}

type pair struct{ Keyword, App string }

// Dataset is an in-memory copy of the recent rank history. All analysis runs
// against it, which keeps the maths testable without a database.
type Dataset struct {
	Apps     []App
	Keywords map[string]Keyword
	series   map[pair][]Point // ascending by time
}

// NewDataset builds an empty dataset (used by Load and by tests).
func NewDataset(apps []App, keywords []Keyword) *Dataset {
	d := &Dataset{Apps: apps, Keywords: map[string]Keyword{}, series: map[pair][]Point{}}
	for _, k := range keywords {
		d.Keywords[k.ID] = k
	}
	return d
}

// Add records one observation. Points may arrive in any order.
func (d *Dataset) Add(keywordID, appID string, at time.Time, rank int) {
	p := pair{keywordID, appID}
	d.series[p] = append(d.series[p], Point{At: at, Rank: rank})
}

// Sort orders every series by time; call it once after the last Add.
func (d *Dataset) Sort() {
	for _, s := range d.series {
		sort.Slice(s, func(i, j int) bool { return s[i].At.Before(s[j].At) })
	}
}

// Mine returns the app marked is_mine (the first one if several are).
func (d *Dataset) Mine() (App, error) {
	for _, a := range d.Apps {
		if a.IsMine {
			return a, nil
		}
	}
	return App{}, errors.New("no tracked app is marked is_mine; set it on one of your tracked_apps")
}

// AppByName finds an app by case-insensitive name; "" means your own app.
func (d *Dataset) AppByName(name string) (App, error) {
	if strings.TrimSpace(name) == "" {
		return d.Mine()
	}
	for _, a := range d.Apps {
		if strings.EqualFold(a.Name, name) {
			return a, nil
		}
	}
	return App{}, fmt.Errorf("no tracked app named %q", name)
}

// Series returns the observations for one keyword/app pair.
func (d *Dataset) Series(keywordID, appID string) []Point { return d.series[pair{keywordID, appID}] }

// rankAt returns the most recent rank observed at or before t.
func rankAt(s []Point, t time.Time) (rank int, ok bool) {
	i := sort.Search(len(s), func(i int) bool { return s[i].At.After(t) })
	if i == 0 {
		return 0, false
	}
	return s[i-1].Rank, true
}

// Load reads tracked apps, tracked keywords and the last HistoryDays of
// rank_history from PocketBase.
func Load(app core.App, asOf time.Time) (*Dataset, error) {
	appRecs, err := app.FindAllRecords("tracked_apps")
	if err != nil {
		return nil, err
	}
	var apps []App
	for _, r := range appRecs {
		id, _ := strconv.ParseInt(r.GetString("track_id"), 10, 64)
		apps = append(apps, App{ID: r.Id, Name: r.GetString("name"), TrackID: id, IsMine: r.GetBool("is_mine")})
	}

	kwRecs, err := app.FindAllRecords("keywords", dbx.HashExp{"tracked": true})
	if err != nil {
		return nil, err
	}
	var kws []Keyword
	for _, r := range kwRecs {
		kws = append(kws, Keyword{ID: r.Id, Term: r.GetString("term"), Country: r.GetString("country")})
	}

	d := NewDataset(apps, kws)

	type row struct {
		Keyword   string `db:"keyword"`
		App       string `db:"app"`
		Rank      int    `db:"rank"`
		CheckedAt string `db:"checked_at"`
	}
	var rows []row
	since := asOf.AddDate(0, 0, -HistoryDays).UTC().Format("2006-01-02 15:04:05.000Z")
	err = app.DB().
		NewQuery("SELECT keyword, app, COALESCE(rank, 0) AS rank, checked_at FROM rank_history WHERE checked_at >= {:since}").
		Bind(dbx.Params{"since": since}).
		All(&rows)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if _, tracked := d.Keywords[r.Keyword]; !tracked {
			continue
		}
		at, err := time.Parse("2006-01-02 15:04:05.000Z", r.CheckedAt)
		if err != nil {
			continue
		}
		d.Add(r.Keyword, r.App, at, r.Rank)
	}
	d.Sort()
	return d, nil
}
