package report

import (
	"math"
	"sort"
	"time"
)

// SparkDays is the width of the daily mini-charts.
const SparkDays = 14

// Altitude scores a rank from 0 (not in the top 100) to 100 (#1). The email
// uses it for bar heights and the visibility score; a mountain metaphor keeps
// "higher is better" obvious.
func Altitude(rank int) int {
	if rank >= 1 && rank <= 100 {
		return 101 - rank
	}
	return 0
}

func effective(rank int) int {
	if rank <= 0 {
		return Unranked
	}
	return rank
}

// Rival is the best-ranked other tracked app for a keyword.
type Rival struct {
	App  string `json:"app"`
	Rank int    `json:"rank"`
}

// KeywordFact is one app's position on one keyword, now versus the start of
// the window. Rank 0 always means "not in the top 200".
type KeywordFact struct {
	KeywordID string `json:"keyword_id"`
	Term      string `json:"term"`
	Country   string `json:"country"`
	Rank      int    `json:"rank"`
	// PrevRank is nil when there is no observation old enough to compare.
	PrevRank *int `json:"prev_rank"`
	// Change is positions gained (+) or lost (-) over the window, treating
	// "unranked" as position 201. Zero when PrevRank is nil.
	Change int `json:"change"`
	// Move is one of: up, down, flat, entered (unranked -> ranked),
	// dropped_out (ranked -> unranked), n/a (no comparison available).
	Move      string `json:"move"`
	BestRival *Rival `json:"best_rival,omitempty"`
	// DailyRanks runs oldest to newest; -1 means the keyword was not tracked yet.
	DailyRanks []int `json:"daily_ranks"`
}

// Facts computes an app's position on every keyword it has data for.
func (d *Dataset) Facts(a App, asOf time.Time, days int) []KeywordFact {
	prevAt := asOf.AddDate(0, 0, -days)
	var out []KeywordFact
	for _, kw := range d.Keywords {
		now, ok := rankAt(d.Series(kw.ID, a.ID), asOf)
		if !ok {
			continue
		}
		f := KeywordFact{KeywordID: kw.ID, Term: kw.Term, Country: kw.Country, Rank: now, Move: "n/a"}

		if prev, ok := rankAt(d.Series(kw.ID, a.ID), prevAt); ok {
			f.PrevRank = &prev
			f.Change = effective(prev) - effective(now)
			switch {
			case prev == 0 && now > 0:
				f.Move = "entered"
			case prev > 0 && now == 0:
				f.Move = "dropped_out"
			case f.Change > 0:
				f.Move = "up"
			case f.Change < 0:
				f.Move = "down"
			default:
				f.Move = "flat"
			}
		}

		for _, other := range d.Apps {
			if other.ID == a.ID {
				continue
			}
			if r, ok := rankAt(d.Series(kw.ID, other.ID), asOf); ok && r > 0 && (f.BestRival == nil || r < f.BestRival.Rank) {
				f.BestRival = &Rival{App: other.Name, Rank: r}
			}
		}

		f.DailyRanks = d.daily(kw.ID, a.ID, asOf, SparkDays)
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Term != out[j].Term {
			return out[i].Term < out[j].Term
		}
		return out[i].Country < out[j].Country
	})
	return out
}

// daily samples a series once per 24h ending at asOf, oldest first.
func (d *Dataset) daily(keywordID, appID string, asOf time.Time, n int) []int {
	s := d.Series(keywordID, appID)
	out := make([]int, n)
	for i := 0; i < n; i++ {
		r, ok := rankAt(s, asOf.Add(-time.Duration(n-1-i)*24*time.Hour))
		if !ok {
			r = -1
		}
		out[i] = r
	}
	return out
}

// Counts are how many keywords an app has in each tier.
type Counts struct {
	Top3   int `json:"top3"`
	Top10  int `json:"top10"`
	Top50  int `json:"top50"`
	Ranked int `json:"ranked"`
}

// Overview is the scoreboard for one app.
type Overview struct {
	App    string `json:"app"`
	IsMine bool   `json:"is_mine"`
	// Country is the storefront this overview is limited to (e.g. "gb"), or
	// empty when it covers every country.
	Country  string `json:"country,omitempty"`
	Keywords int    `json:"keywords"`
	// Visibility is the mean altitude over all keywords, 0-100.
	Visibility float64 `json:"visibility"`
	// VisibilityChange compares like with like: only keywords observed both
	// now and at the start of the window. Nil when nothing is comparable.
	VisibilityChange *float64 `json:"visibility_change"`
	Now              Counts   `json:"now"`
	Prev             *Counts  `json:"prev"`
	AvgRank          float64  `json:"avg_rank_when_ranked"`
	// DailyVisibility has SparkDays entries, oldest first.
	DailyVisibility []float64 `json:"daily_visibility"`
}

func round1(f float64) float64 { return math.Round(f*10) / 10 }

func tally(c *Counts, rank int) {
	if rank <= 0 {
		return
	}
	c.Ranked++
	if rank <= 50 {
		c.Top50++
	}
	if rank <= 10 {
		c.Top10++
	}
	if rank <= 3 {
		c.Top3++
	}
}

// Overview summarises an app across all its keywords, in every country.
func (d *Dataset) Overview(a App, asOf time.Time, days int) Overview {
	return overviewOf(a, d.Facts(a, asOf, days))
}

// CountryOverviews splits an app's overview by storefront, sorted by country
// code, so a weak new market can't hide inside (or drag down) a strong one.
// It returns nil when the app is tracked in fewer than two countries, where
// the split would only repeat Overview.
func (d *Dataset) CountryOverviews(a App, asOf time.Time, days int) []Overview {
	byCountry := map[string][]KeywordFact{}
	for _, f := range d.Facts(a, asOf, days) {
		byCountry[f.Country] = append(byCountry[f.Country], f)
	}
	if len(byCountry) < 2 {
		return nil
	}
	countries := make([]string, 0, len(byCountry))
	for c := range byCountry {
		countries = append(countries, c)
	}
	sort.Strings(countries)

	out := make([]Overview, 0, len(countries))
	for _, c := range countries {
		o := overviewOf(a, byCountry[c])
		o.Country = c
		out = append(out, o)
	}
	return out
}

// overviewOf summarises already-computed facts for one app.
func overviewOf(a App, facts []KeywordFact) Overview {
	o := Overview{App: a.Name, IsMine: a.IsMine, Keywords: len(facts)}

	var sumAlt, rankSum float64
	var pairedNow, pairedPrev float64
	var paired int
	var prev Counts
	for _, f := range facts {
		sumAlt += float64(Altitude(f.Rank))
		tally(&o.Now, f.Rank)
		if f.Rank > 0 {
			rankSum += float64(f.Rank)
		}
		if f.PrevRank != nil {
			paired++
			pairedNow += float64(Altitude(f.Rank))
			pairedPrev += float64(Altitude(*f.PrevRank))
			tally(&prev, *f.PrevRank)
		}
	}
	if len(facts) > 0 {
		o.Visibility = round1(sumAlt / float64(len(facts)))
	}
	if o.Now.Ranked > 0 {
		o.AvgRank = round1(rankSum / float64(o.Now.Ranked))
	}
	if paired > 0 {
		o.Prev = &prev
		c := round1(pairedNow/float64(paired) - pairedPrev/float64(paired))
		o.VisibilityChange = &c
	}

	o.DailyVisibility = make([]float64, SparkDays)
	for i := range o.DailyVisibility {
		var sum float64
		var n int
		for _, f := range facts {
			if r := f.DailyRanks[i]; r >= 0 {
				sum += float64(Altitude(r))
				n++
			}
		}
		if n > 0 {
			o.DailyVisibility[i] = round1(sum / float64(n))
		}
	}
	return o
}

// Movers returns the biggest gains (dir "up") or losses (dir "down").
func Movers(facts []KeywordFact, dir string, limit int) []KeywordFact {
	var out []KeywordFact
	for _, f := range facts {
		if (dir == "up" && f.Change > 0) || (dir == "down" && f.Change < 0) {
			out = append(out, f)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Change != out[j].Change {
			if dir == "up" {
				return out[i].Change > out[j].Change
			}
			return out[i].Change < out[j].Change
		}
		return effective(out[i].Rank) < effective(out[j].Rank)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// Gap is a keyword where a competitor is in the top tier and you are lower.
type Gap struct {
	KeywordFact
	// Gap is how many positions you trail the best rival by.
	Gap int `json:"gap"`
}

// Opportunities groups the keywords most worth working on.
type Opportunities struct {
	// StrikingDistance: ranked 11-30, one push from the first page of results.
	StrikingDistance []KeywordFact `json:"striking_distance"`
	// PodiumPush: ranked 4-10, close to the top 3.
	PodiumPush []KeywordFact `json:"podium_push"`
	// Gaps: a competitor is in the top 10 and you rank lower or not at all.
	Gaps []Gap `json:"competitor_gaps"`
}

// FindOpportunities scans facts for the lists in Opportunities.
func FindOpportunities(facts []KeywordFact, limit int) Opportunities {
	var o Opportunities
	for _, f := range facts {
		switch {
		case f.Rank >= 11 && f.Rank <= 30:
			o.StrikingDistance = append(o.StrikingDistance, f)
		case f.Rank >= 4 && f.Rank <= 10:
			o.PodiumPush = append(o.PodiumPush, f)
		}
		if f.BestRival != nil && f.BestRival.Rank <= 10 && effective(f.Rank) > f.BestRival.Rank {
			o.Gaps = append(o.Gaps, Gap{KeywordFact: f, Gap: effective(f.Rank) - f.BestRival.Rank})
		}
	}
	byRank := func(s []KeywordFact) {
		sort.SliceStable(s, func(i, j int) bool { return s[i].Rank < s[j].Rank })
	}
	byRank(o.StrikingDistance)
	byRank(o.PodiumPush)
	sort.SliceStable(o.Gaps, func(i, j int) bool { return o.Gaps[i].Gap > o.Gaps[j].Gap })

	if limit > 0 {
		if len(o.StrikingDistance) > limit {
			o.StrikingDistance = o.StrikingDistance[:limit]
		}
		if len(o.PodiumPush) > limit {
			o.PodiumPush = o.PodiumPush[:limit]
		}
		if len(o.Gaps) > limit {
			o.Gaps = o.Gaps[:limit]
		}
	}
	return o
}

// RivalReport is one competitor's week.
type RivalReport struct {
	Overview Overview      `json:"overview"`
	Climbs   []KeywordFact `json:"biggest_climbs"`
	Slips    []KeywordFact `json:"biggest_slips"`
}

// RivalReports summarises every competitor.
func (d *Dataset) RivalReports(asOf time.Time, days, limit int) []RivalReport {
	var out []RivalReport
	for _, a := range d.Apps {
		if a.IsMine {
			continue
		}
		facts := d.Facts(a, asOf, days)
		out = append(out, RivalReport{
			Overview: d.Overview(a, asOf, days),
			Climbs:   Movers(facts, "up", limit),
			Slips:    Movers(facts, "down", limit),
		})
	}
	return out
}

// AppHistory is one app's daily ranks for a keyword.
type AppHistory struct {
	App        string `json:"app"`
	IsMine     bool   `json:"is_mine"`
	DailyRanks []int  `json:"daily_ranks"`
}

// History returns every tracked app's daily ranks on one keyword.
func (d *Dataset) History(keywordID string, asOf time.Time, days int) []AppHistory {
	var out []AppHistory
	for _, a := range d.Apps {
		out = append(out, AppHistory{App: a.Name, IsMine: a.IsMine, DailyRanks: d.daily(keywordID, a.ID, asOf, days)})
	}
	return out
}

// Snapshot is everything the renderer needs: your scoreboard, rivals, and a
// fact per keyword so Content can reference keywords by ID.
type Snapshot struct {
	AsOf time.Time
	Days int
	Mine App
	Me   Overview
	// MyCountries is Me split by storefront; nil with fewer than two countries.
	MyCountries []Overview
	Rivals      []Overview
	Facts       map[string]KeywordFact
}

// Snapshot builds the Snapshot for the window ending at asOf.
func (d *Dataset) Snapshot(asOf time.Time, days int) (*Snapshot, error) {
	mine, err := d.Mine()
	if err != nil {
		return nil, err
	}
	s := &Snapshot{AsOf: asOf, Days: days, Mine: mine, Me: d.Overview(mine, asOf, days),
		MyCountries: d.CountryOverviews(mine, asOf, days), Facts: map[string]KeywordFact{}}
	for _, f := range d.Facts(mine, asOf, days) {
		s.Facts[f.KeywordID] = f
	}
	for _, a := range d.Apps {
		if !a.IsMine {
			s.Rivals = append(s.Rivals, d.Overview(a, asOf, days))
		}
	}
	return s, nil
}
