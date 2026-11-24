package report

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

// sampleRow scripts one keyword: where each app was ~10 days ago and where it
// is now. Index 0 is "Habitual" (you), then three rivals.
type sampleRow struct {
	term       string
	start, end [4]int
}

var sampleRows = []sampleRow{
	{"habit tracker", [4]int{31, 5, 9, 22}, [4]int{12, 4, 9, 25}},
	{"daily routine", [4]int{18, 6, 2, 0}, [4]int{9, 6, 3, 0}},
	{"streaks", [4]int{12, 4, 20, 31}, [4]int{3, 8, 21, 30}},
	{"routine builder", [4]int{3, 11, 0, 40}, [4]int{1, 12, 0, 38}},
	{"mood journal", [4]int{0, 2, 0, 0}, [4]int{150, 2, 0, 0}},
	{"habit streak app", [4]int{8, 3, 12, 0}, [4]int{15, 2, 11, 0}},
	{"goal tracker", [4]int{5, 1, 14, 9}, [4]int{6, 1, 15, 9}},
	{"self care", [4]int{44, 12, 0, 0}, [4]int{28, 11, 0, 0}},
	{"morning routine", [4]int{22, 7, 5, 0}, [4]int{21, 7, 5, 0}},
	{"daily planner", [4]int{60, 0, 4, 2}, [4]int{57, 0, 4, 2}},
	{"to do list", [4]int{0, 15, 3, 1}, [4]int{0, 15, 3, 1}},
	{"build habits", [4]int{14, 8, 0, 33}, [4]int{11, 9, 0, 33}},
	{"water reminder", [4]int{7, 25, 0, 0}, [4]int{24, 6, 0, 0}},
	{"mindfulness", [4]int{33, 0, 0, 18}, [4]int{0, 0, 0, 17}},
	{"gratitude journal", [4]int{70, 3, 0, 0}, [4]int{38, 3, 0, 0}},
	{"habit coach", [4]int{26, 14, 0, 0}, [4]int{19, 14, 0, 0}},
	{"sleep tracker", [4]int{0, 0, 0, 0}, [4]int{0, 0, 0, 0}},
	{"focus timer", [4]int{47, 0, 22, 0}, [4]int{46, 0, 18, 0}},
}

// SampleNow is a fixed reference time so sample output is reproducible.
var SampleNow = time.Date(2026, 11, 18, 14, 0, 0, 0, time.UTC)

// SampleDataset builds a realistic, deterministic dataset ending at now:
// one app of yours and three rivals over 35 days of daily checks.
func SampleDataset(now time.Time) *Dataset {
	apps := []App{
		{ID: "a0", Name: "Habitual", TrackID: 1, IsMine: true},
		{ID: "a1", Name: "Streakly", TrackID: 2},
		{ID: "a2", Name: "DailyDo", TrackID: 3},
		{ID: "a3", Name: "Loop Habit", TrackID: 4},
	}
	var kws []Keyword
	for i, r := range sampleRows {
		country := "us"
		if i >= 12 { // the last six are a newer second storefront, so the by-country table has something to show
			country = "gb"
		}
		kws = append(kws, Keyword{ID: fmt.Sprintf("kw%02d", i+1), Term: r.term, Country: country})
	}
	d := NewDataset(apps, kws)

	rng := rand.New(rand.NewSource(7))
	base := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, time.UTC)
	if base.After(now) {
		base = base.AddDate(0, 0, -1)
	}
	const shiftDays = 7 // the scripted change plays out over exactly the report window
	for i, r := range sampleRows {
		for ai := range apps {
			for ago := HistoryDays - 1; ago >= 0; ago-- {
				d.Add(kws[i].ID, apps[ai].ID, base.AddDate(0, 0, -ago), sampleRank(rng, r.start[ai], r.end[ai], ago, shiftDays))
			}
		}
	}
	d.Sort()
	return d
}

func sampleRank(rng *rand.Rand, from, to, ago, shift int) int {
	wobble := rng.Intn(3) - 1
	if ago == 0 || ago == shift { // the two ends of the window are exact
		wobble = 0
	}
	switch {
	case from == to:
		if from == 0 {
			return 0
		}
		return max(1, from+wobble)
	case from == 0 || to == 0: // appearing or vanishing is a step, not a slope
		if ago > 2 {
			return from
		}
		return to
	case ago >= shift:
		return max(1, from+wobble)
	default:
		frac := float64(shift-ago) / float64(shift)
		return max(1, from+int(math.Round(float64(to-from)*frac))+wobble)
	}
}

// SampleContent is hand-written copy that matches SampleDataset, used for
// design previews and tests. It costs nothing to render.
func SampleContent() *Content {
	return &Content{
		Subject:        "Streaks hit #3, and one keyword fell off a cliff",
		Preheader:      "A week of big climbs, one surprise rival surge, and five quests to bank the gains.",
		Mood:           "climbing",
		Headline:       "Streaks hit the summit. Water reminder slid down the far side.",
		Intro:          "A strong week overall: your visibility score is up, \"streaks\" broke into the top 3 and \"routine builder\" took the number one spot. The bad news is hiding in the foothills, where Streakly quietly rewrote its way past you on \"water reminder\".",
		ScoreboardNote: "The summit count doubled, while two ridge regulars slipped out the back.",
		Highlights: []Highlight{
			{Kind: "milestone", Title: "🏆 You own \"routine builder\"", Body: "Number one, with Streakly a comfortable eleven places behind. Protect it by keeping that phrase in your title and shipping on schedule.", KeywordID: "kw04"},
			{Kind: "win", Title: "🚀 \"streaks\" jumped into the top 3", Body: "Nine places in a week, and Streakly slipped four places on the same term. The gap is now yours to defend.", KeywordID: "kw03"},
			{Kind: "surprise", Title: "✨ Fresh ground on \"mood journal\"", Body: "You appeared out of nowhere at #150. Not glamorous, but Streakly sits at #2, so the demand is there and the first foothold is in.", KeywordID: "kw05"},
			{Kind: "loss", Title: "📉 \"water reminder\" fell 17 places", Body: "Streakly climbed from #25 to #6 over the same days. That looks like a metadata change on their side, worth a look at their new listing.", KeywordID: "kw13"},
		},
		Climbers: Section{
			Intro: "The ones pulling the score up.",
			Items: []Pick{
				{KeywordID: "kw01", Comment: "Nineteen places in a week. Two more like it and you cross into the top 10."},
				{KeywordID: "kw02", Comment: "Steady pull toward the ridge. DailyDo is #3, so the podium is the next target."},
				{KeywordID: "kw15", Comment: "Long tail, low competition. Cheap places to win."},
			},
		},
		Fallers: Section{
			Intro: "Not panic territory, but watch these.",
			Items: []Pick{
				{KeywordID: "kw13", Comment: "The big one. Compare your subtitle with Streakly's before touching anything."},
				{KeywordID: "kw14", Comment: "You were #33, now off the chart. Mindfulness is crowded, so decide if it's worth fighting for."},
				{KeywordID: "kw06", Comment: "Seven places down while Streakly moved up. They are targeting the same phrase."},
			},
		},
		Rivals: Corner{
			Title: "Streakly is on the move",
			Body:  "The only competitor with real momentum this week. They gained on three keywords you care about and lost ground on one.",
			Callouts: []Callout{
				{App: "Streakly", Note: "Jumped from #25 to #6 on \"water reminder\". Probably a metadata update.", KeywordID: "kw13"},
				{App: "DailyDo", Note: "Holds #3 and #4 on the broad planner terms and hasn't moved. Safe to ignore this week.", KeywordID: "kw02"},
			},
		},
		Quests: []Quest{
			{Title: "Reclaim \"water reminder\" before it settles", Why: "A 17 place drop on a term you ranked #7 for, with a competitor who is clearly targeting it.", How: "Read Streakly's new title and subtitle, then work \"water\" back into your own subtitle in the next release. Check the rank again in seven days.", Effort: "medium", Impact: "high", KeywordIDs: []string{"kw13"}},
			{Title: "Push \"habit tracker\" over the line", Why: "You're #12 and climbing, one tier from the first page of results.", How: "Reply to your five most recent reviews and ask happy users for a rating in-app after a streak milestone.", Effort: "quick", Impact: "medium", KeywordIDs: []string{"kw01"}},
			{Title: "Use your new foothold on \"mood journal\"", Why: "Streakly is #2 on a term you just entered at #150. The demand is proven.", How: "Add \"mood\" to the keyword field (no spaces, comma separated) and mention mood tracking in your first screenshot caption.", Effort: "quick", Impact: "medium", KeywordIDs: []string{"kw05"}},
			{Title: "Decide about mindfulness", Why: "You dropped out of the top 200 on a competitive term.", How: "Either commit with a dedicated screenshot and subtitle mention, or free the keyword field space for terms where you rank.", Effort: "big", Impact: "low", KeywordIDs: []string{"kw14"}},
		},
		SignOff: "That's the climb for this week. Same time next Monday, and bring your crampons.",
	}
}
