package report

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 18, 14, 0, 0, 0, time.UTC)

// fixture: daysAgo -> rank, per keyword/app. Checks happen at 02:00 daily.
func add(d *Dataset, kw, app string, ranks map[int]int) {
	for ago, r := range ranks {
		d.Add(kw, app, t0.Add(-12*time.Hour).AddDate(0, 0, -ago), r)
	}
}

func fixture() *Dataset {
	d := NewDataset(
		[]App{{ID: "me", Name: "Mine", IsMine: true}, {ID: "rv", Name: "Rival"}},
		[]Keyword{{ID: "k1", Term: "habit tracker", Country: "us"}, {ID: "k2", Term: "mood journal", Country: "us"},
			{ID: "k3", Term: "streaks", Country: "us"}, {ID: "k4", Term: "brand new", Country: "us"}},
	)
	// k1: 30 -> 12 over the week (climb, striking distance), rival #4
	add(d, "k1", "me", map[int]int{10: 31, 8: 30, 7: 30, 3: 20, 0: 12})
	add(d, "k1", "rv", map[int]int{10: 5, 7: 5, 0: 4})
	// k2: unranked a week ago, now #150 (entered); rival #2
	add(d, "k2", "me", map[int]int{8: 0, 7: 0, 0: 150})
	add(d, "k2", "rv", map[int]int{8: 2, 0: 2})
	// k3: #3 -> #9 (down), no rival
	add(d, "k3", "me", map[int]int{8: 3, 7: 3, 0: 9})
	// k4: first observed yesterday: no comparison possible
	add(d, "k4", "me", map[int]int{1: 40, 0: 38})
	d.Sort()
	return d
}

func TestFacts(t *testing.T) {
	d := fixture()
	me, _ := d.Mine()
	byID := map[string]KeywordFact{}
	for _, f := range d.Facts(me, t0, 7) {
		byID[f.KeywordID] = f
	}

	k1 := byID["k1"]
	if k1.Rank != 12 || k1.PrevRank == nil || *k1.PrevRank != 30 || k1.Change != 18 || k1.Move != "up" {
		t.Errorf("k1 = %+v", k1)
	}
	if k1.BestRival == nil || k1.BestRival.App != "Rival" || k1.BestRival.Rank != 4 {
		t.Errorf("k1 rival = %+v", k1.BestRival)
	}

	k2 := byID["k2"]
	if k2.Move != "entered" || k2.Change != Unranked-150 {
		t.Errorf("k2 = %+v", k2)
	}

	if k3 := byID["k3"]; k3.Move != "down" || k3.Change != -6 {
		t.Errorf("k3 = %+v", k3)
	}

	k4 := byID["k4"]
	if k4.Move != "n/a" || k4.PrevRank != nil || k4.Change != 0 {
		t.Errorf("k4 = %+v", k4)
	}
	if got := k4.DailyRanks; got[0] != -1 || got[SparkDays-1] != 38 {
		t.Errorf("k4 daily = %v", got)
	}
}

func TestOverview(t *testing.T) {
	d := fixture()
	me, _ := d.Mine()
	o := d.Overview(me, t0, 7)

	if o.Keywords != 4 || o.Now.Top10 != 1 || o.Now.Top3 != 0 || o.Now.Ranked != 4 {
		t.Errorf("counts = %+v", o)
	}
	if o.Prev == nil || o.Prev.Top3 != 1 || o.Prev.Ranked != 2 {
		t.Errorf("prev = %+v", o.Prev)
	}
	// Paired keywords (k1,k2,k3): altitude now 89+0+92 vs prev 71+0+98.
	// (181-169)/3 = 4.0
	if o.VisibilityChange == nil || *o.VisibilityChange != 4.0 {
		t.Errorf("visibility change = %v", o.VisibilityChange)
	}
	if len(o.DailyVisibility) != SparkDays {
		t.Errorf("daily len = %d", len(o.DailyVisibility))
	}
}

func TestMoversAndOpportunities(t *testing.T) {
	d := fixture()
	me, _ := d.Mine()
	facts := d.Facts(me, t0, 7)

	up := Movers(facts, "up", 5)
	if len(up) != 2 || up[0].KeywordID != "k2" { // entering the top 200 is the largest jump
		t.Errorf("up = %+v", up)
	}
	down := Movers(facts, "down", 5)
	if len(down) != 1 || down[0].KeywordID != "k3" {
		t.Errorf("down = %+v", down)
	}

	o := FindOpportunities(facts, 10)
	if len(o.StrikingDistance) != 1 || o.StrikingDistance[0].KeywordID != "k1" {
		t.Errorf("striking = %+v", o.StrikingDistance)
	}
	if len(o.PodiumPush) != 1 || o.PodiumPush[0].KeywordID != "k3" {
		t.Errorf("podium = %+v", o.PodiumPush)
	}
	// k2: rival #2, me #150 => gap 148 (largest); k1: rival #4, me #12 => gap 8.
	if len(o.Gaps) != 2 || o.Gaps[0].KeywordID != "k2" || o.Gaps[0].Gap != 148 || o.Gaps[1].Gap != 8 {
		t.Errorf("gaps = %+v", o.Gaps)
	}
}

func TestSnapshotRequiresMine(t *testing.T) {
	d := NewDataset([]App{{ID: "a", Name: "A"}}, nil)
	if _, err := d.Snapshot(t0, 7); err == nil {
		t.Fatal("expected error when no app is_mine")
	}
}

func TestCountryOverviews(t *testing.T) {
	// Single country: the split would only repeat Overview.
	d := fixture()
	me, _ := d.Mine()
	if got := d.CountryOverviews(me, t0, 7); got != nil {
		t.Errorf("single country should give nil, got %+v", got)
	}

	// us: #1 -> altitude 100. gb: unranked -> altitude 0. A blended score hides both.
	d = NewDataset(
		[]App{{ID: "me", Name: "Mine", IsMine: true}},
		[]Keyword{
			{ID: "u1", Term: "learn german", Country: "us"},
			{ID: "g1", Term: "learn german", Country: "gb"},
			{ID: "g2", Term: "gcse german", Country: "gb"},
		},
	)
	add(d, "u1", "me", map[int]int{8: 1, 7: 1, 0: 1})
	add(d, "g1", "me", map[int]int{8: 0, 7: 0, 0: 0})
	add(d, "g2", "me", map[int]int{8: 0, 7: 0, 0: 11})
	d.Sort()
	me, _ = d.Mine()

	blended := d.Overview(me, t0, 7)
	if blended.Country != "" || blended.Keywords != 3 {
		t.Errorf("blended = %+v", blended)
	}

	got := d.CountryOverviews(me, t0, 7)
	if len(got) != 2 || got[0].Country != "gb" || got[1].Country != "us" {
		t.Fatalf("countries = %+v", got)
	}
	gb, us := got[0], got[1]
	if gb.Keywords != 2 || gb.Visibility != 45.0 { // (0 + 90) / 2
		t.Errorf("gb = %+v", gb)
	}
	if us.Keywords != 1 || us.Visibility != 100.0 {
		t.Errorf("us = %+v", us)
	}
	// gb gained 90 altitude on g2 over 2 paired keywords; us unchanged.
	if gb.VisibilityChange == nil || *gb.VisibilityChange != 45.0 {
		t.Errorf("gb change = %v", gb.VisibilityChange)
	}
	if us.VisibilityChange == nil || *us.VisibilityChange != 0 {
		t.Errorf("us change = %v", us.VisibilityChange)
	}
	if gb.Now.Top10 != 0 || gb.Now.Ranked != 1 || us.Now.Top3 != 1 {
		t.Errorf("counts gb=%+v us=%+v", gb.Now, us.Now)
	}

	snap, err := d.Snapshot(t0, 7)
	if err != nil || len(snap.MyCountries) != 2 {
		t.Errorf("snapshot countries = %+v (%v)", snap.MyCountries, err)
	}
}
