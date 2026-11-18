package report

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"strings"
)

//go:embed email.html.tmpl
var emailTmpl string

var tmpl = template.Must(template.New("email").Parse(emailTmpl))

// Tier colours: the altitude of a rank, used for chips and chart bars.
const (
	colSummit  = "#E8A317" // top 3
	colRidge   = "#2F9E6B" // top 10
	colFoot    = "#3B82C4" // top 30
	colBase    = "#B7B1A6" // everything else
	colNoData  = "#E4DFD6"
	colUp      = "#1E7F4F"
	colUpBg    = "#E1F3E8"
	colDown    = "#B3261E"
	colDownBg  = "#FBE4E1"
	colFlat    = "#6B665C"
	colFlatBg  = "#ECE8E0"
	colNewFg   = "#7A4B00"
	colNewBg   = "#FFF0CC"
	maxBarPx   = 28
	minBarPx   = 3
	heroBarMax = 70
	heroBarMin = 10
)

type moodStyle struct {
	Label, Emoji     string
	From, To, Accent template.CSS
}

var moodStyles = map[string]moodStyle{
	"summit":   {"Summit week", "🏔️", "#F59E0B", "#DC4C1B", "#E8A317"},
	"climbing": {"On the climb", "🧗", "#0F766E", "#2F9E6B", "#2F9E6B"},
	"steady":   {"Steady ground", "⛺", "#335F8A", "#5B8DB8", "#3B82C4"},
	"slipping": {"Losing altitude", "🪨", "#9A5B13", "#C2410C", "#C2410C"},
	"storm":    {"Storm warning", "⛈️", "#1F2937", "#4B5563", "#B3261E"},
}

// Meta is non-ranking context for the email chrome.
type Meta struct {
	IconURL string // optional app icon
}

// Email is the finished message.
type Email struct {
	Subject string
	HTML    string
	Text    string
}

type bar struct {
	H     int
	Color template.CSS
	Alpha template.CSS // opacity for hero bars, "" otherwise
}

type chip struct {
	Text   string
	Fg, Bg template.CSS
}

type tile struct {
	Label, Value, Sub string
	Delta             chip
}

type rowView struct {
	Term, Country, Rank string
	RankColor           template.CSS
	Change              chip
	Detail              string
	Spark               []bar
	Comment             string
}

type highlightView struct {
	Emoji, Title, Body string
	Border             template.CSS
	Row                *rowView
}

type sectionView struct {
	Title, Intro string
	Rows         []rowView
}

type calloutView struct {
	App, Note string
	Row       *rowView
}

type questView struct {
	N                                      int
	Title, Why, How                        string
	Effort, Impact                         string
	EffortFg, EffortBg, ImpactFg, ImpactBg template.CSS
	Rows                                   []rowView
}

type view struct {
	Subject, Preheader    string
	Mood                  moodStyle
	Week, AppName         string
	IconURL               string
	Headline, Intro       string
	ScoreNote             string
	Hero                  []bar
	Tiles                 []tile
	Highlights            []highlightView
	Climbers, Fallers     sectionView
	RivalTitle, RivalBody string
	Callouts              []calloutView
	Quests                []questView
	SignOff               string
	Footer                string
}

func tierColor(rank int) template.CSS {
	switch {
	case rank >= 1 && rank <= 3:
		return colSummit
	case rank >= 1 && rank <= 10:
		return colRidge
	case rank >= 1 && rank <= 30:
		return colFoot
	default:
		return colBase
	}
}

func rankText(rank int) string {
	if rank <= 0 {
		return "200+"
	}
	return fmt.Sprintf("#%d", rank)
}

func scaleBar(altitude int) int {
	return minBarPx + (maxBarPx-minBarPx)*altitude/100
}

func sparkBars(daily []int) []bar {
	out := make([]bar, len(daily))
	for i, r := range daily {
		switch {
		case r < 0:
			out[i] = bar{H: 1, Color: colNoData}
		case r == 0:
			out[i] = bar{H: minBarPx, Color: colNoData}
		default:
			out[i] = bar{H: scaleBar(Altitude(r)), Color: tierColor(r)}
		}
	}
	return out
}

// heroBars draws the daily visibility series as a skyline, scaled to its own
// range so small week-to-week changes still show.
func heroBars(series []float64) []bar {
	lo, hi := series[0], series[0]
	for _, v := range series {
		lo, hi = min(lo, v), max(hi, v)
	}
	out := make([]bar, len(series))
	for i, v := range series {
		frac := 0.5
		if hi > lo {
			frac = (v - lo) / (hi - lo)
		}
		a := template.CSS("0.38")
		if i == len(series)-1 {
			a = "1"
		}
		out[i] = bar{H: heroBarMin + int(frac*float64(heroBarMax-heroBarMin)), Color: "#FFFFFF", Alpha: a}
	}
	return out
}

func deltaChip(change int, hasPrev bool) chip {
	switch {
	case !hasPrev:
		return chip{"first week", colFlat, colFlatBg}
	case change > 0:
		return chip{fmt.Sprintf("▲ %d", change), colUp, colUpBg}
	case change < 0:
		return chip{fmt.Sprintf("▼ %d", -change), colDown, colDownBg}
	default:
		return chip{"no change", colFlat, colFlatBg}
	}
}

func rowFor(f KeywordFact) rowView {
	r := rowView{Term: f.Term, Country: strings.ToUpper(f.Country), Rank: rankText(f.Rank),
		RankColor: tierColor(f.Rank), Spark: sparkBars(f.DailyRanks)}
	prev := ""
	if f.PrevRank != nil {
		prev = rankText(*f.PrevRank)
	}
	switch f.Move {
	case "up":
		r.Change, r.Detail = chip{fmt.Sprintf("▲ %d", f.Change), colUp, colUpBg}, "from "+prev
	case "down":
		r.Change, r.Detail = chip{fmt.Sprintf("▼ %d", -f.Change), colDown, colDownBg}, "from "+prev
	case "entered":
		r.Change, r.Detail = chip{"NEW", colNewFg, colNewBg}, "entered the top 200"
	case "dropped_out":
		r.Change, r.Detail = chip{"OUT", colDown, colDownBg}, "was "+prev
	case "flat":
		r.Change, r.Detail = chip{"steady", colFlat, colFlatBg}, "unchanged"
	default:
		r.Change, r.Detail = chip{"new", colNewFg, colNewBg}, "just started tracking"
	}
	if f.BestRival != nil {
		r.Detail += fmt.Sprintf(" · %s is %s", f.BestRival.App, rankText(f.BestRival.Rank))
	}
	return r
}

func countDelta(now int, prev *Counts, pick func(Counts) int) chip {
	if prev == nil {
		return deltaChip(0, false)
	}
	return deltaChip(now-pick(*prev), true)
}

func buildView(c *Content, s *Snapshot, m Meta) view {
	mood := moodStyles[c.Mood]
	if mood.Label == "" {
		mood = moodStyles["steady"]
	}
	me := s.Me
	v := view{
		Subject: c.Subject, Preheader: c.Preheader, Mood: mood, AppName: s.Mine.Name, IconURL: m.IconURL,
		Week:     fmt.Sprintf("%s – %s", s.AsOf.AddDate(0, 0, -s.Days).Format("Jan 2"), s.AsOf.Format("Jan 2, 2006")),
		Headline: c.Headline, Intro: c.Intro, ScoreNote: c.ScoreboardNote,
		Hero:    heroBars(me.DailyVisibility),
		SignOff: c.SignOff,
		Footer: fmt.Sprintf("Tracking %d keywords across %d competitors. Ranks come from live App Store search results.",
			me.Keywords, len(s.Rivals)),
	}

	vis := deltaChip(0, false)
	if me.VisibilityChange != nil {
		switch ch := *me.VisibilityChange; {
		case ch > 0:
			vis = chip{fmt.Sprintf("▲ %.1f", ch), colUp, colUpBg}
		case ch < 0:
			vis = chip{fmt.Sprintf("▼ %.1f", -ch), colDown, colDownBg}
		default:
			vis = chip{"no change", colFlat, colFlatBg}
		}
	}
	v.Tiles = []tile{
		{"Visibility", fmt.Sprintf("%.1f", me.Visibility), "out of 100", vis},
		{"On the ridge", fmt.Sprint(me.Now.Top10), "keywords in the top 10", countDelta(me.Now.Top10, me.Prev, func(c Counts) int { return c.Top10 })},
		{"At the summit", fmt.Sprint(me.Now.Top3), "keywords in the top 3", countDelta(me.Now.Top3, me.Prev, func(c Counts) int { return c.Top3 })},
	}

	row := func(id string) *rowView {
		if f, ok := s.Facts[id]; ok {
			r := rowFor(f)
			return &r
		}
		return nil
	}

	emoji := map[string]string{"win": "🚀", "loss": "📉", "surprise": "✨", "milestone": "🏆"}
	border := map[string]template.CSS{"win": colUp, "loss": colDown, "surprise": colFoot, "milestone": colSummit}
	for _, h := range c.Highlights {
		hv := highlightView{Emoji: emoji[h.Kind], Title: stripLeadingEmoji(h.Title), Body: h.Body, Border: border[h.Kind]}
		if h.KeywordID != "" {
			hv.Row = row(h.KeywordID)
		}
		v.Highlights = append(v.Highlights, hv)
	}

	section := func(title string, sec Section) sectionView {
		sv := sectionView{Title: title, Intro: sec.Intro}
		for _, p := range sec.Items {
			if r := row(p.KeywordID); r != nil {
				r.Comment = p.Comment
				sv.Rows = append(sv.Rows, *r)
			}
		}
		return sv
	}
	v.Climbers = section("On the way up", c.Climbers)
	v.Fallers = section("Losing altitude", c.Fallers)

	v.RivalTitle, v.RivalBody = c.Rivals.Title, c.Rivals.Body
	for _, k := range c.Rivals.Callouts {
		cv := calloutView{App: k.App, Note: k.Note}
		if k.KeywordID != "" {
			cv.Row = row(k.KeywordID)
		}
		v.Callouts = append(v.Callouts, cv)
	}

	effort := map[string]string{"quick": "Quick win", "medium": "Half a day", "big": "Bigger project"}
	for i, q := range c.Quests {
		qv := questView{N: i + 1, Title: q.Title, Why: q.Why, How: q.How,
			Effort: effort[q.Effort], Impact: impactLabel(q.Impact)}
		qv.EffortFg, qv.EffortBg = colFlat, colFlatBg
		switch q.Impact {
		case "high":
			qv.ImpactFg, qv.ImpactBg = colUp, colUpBg
		case "medium":
			qv.ImpactFg, qv.ImpactBg = "#1F5F94", "#E3EFFA"
		default:
			qv.ImpactFg, qv.ImpactBg = colFlat, colFlatBg
		}
		for _, id := range q.KeywordIDs {
			if r := row(id); r != nil {
				qv.Rows = append(qv.Rows, *r)
			}
		}
		v.Quests = append(v.Quests, qv)
	}
	return v
}

func impactLabel(impact string) string {
	if impact == "" {
		return "Medium impact"
	}
	return strings.ToUpper(impact[:1]) + impact[1:] + " impact"
}

// stripLeadingEmoji removes a leading pictograph the model may add, since the
// template supplies its own per-kind icon.
func stripLeadingEmoji(s string) string {
	s = strings.TrimSpace(s)
	for i, r := range s {
		if r < 0x2000 { // first plain letter, digit or punctuation
			return strings.TrimSpace(s[i:])
		}
	}
	return s
}

// Render builds the HTML and plain-text email.
func Render(c *Content, s *Snapshot, m Meta) (*Email, error) {
	v := buildView(c, s, m)
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, v); err != nil {
		return nil, err
	}
	return &Email{Subject: c.Subject, HTML: buf.String(), Text: renderText(v)}, nil
}

func renderText(v view) string {
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }
	rows := func(rs []rowView) {
		for _, r := range rs {
			w("  %s  %s (%s) %s, %s", r.Rank, r.Term, r.Country, r.Change.Text, r.Detail)
			if r.Comment != "" {
				w("      %s", r.Comment)
			}
		}
	}

	w("THE WEEKLY CLIMB · %s · %s", v.AppName, v.Week)
	w("%s %s", strings.ToUpper(v.Mood.Label), "")
	w("%s\n", v.Headline)
	w("%s\n", v.Intro)
	for _, t := range v.Tiles {
		w("%s: %s (%s) %s", t.Label, t.Value, t.Sub, t.Delta.Text)
	}
	if v.ScoreNote != "" {
		w("%s", v.ScoreNote)
	}
	b.WriteString("\n")
	for _, h := range v.Highlights {
		w("%s %s\n%s", h.Emoji, h.Title, h.Body)
		if h.Row != nil {
			rows([]rowView{*h.Row})
		}
		b.WriteString("\n")
	}
	for _, sec := range []sectionView{v.Climbers, v.Fallers} {
		if len(sec.Rows) == 0 {
			continue
		}
		w("-- %s --", strings.ToUpper(sec.Title))
		if sec.Intro != "" {
			w("%s", sec.Intro)
		}
		rows(sec.Rows)
		b.WriteString("\n")
	}
	if v.RivalTitle != "" || len(v.Callouts) > 0 {
		w("-- %s --", strings.ToUpper(v.RivalTitle))
		w("%s", v.RivalBody)
		for _, c := range v.Callouts {
			w("* %s: %s", c.App, c.Note)
		}
		b.WriteString("\n")
	}
	w("-- THIS WEEK'S QUESTS --")
	for _, q := range v.Quests {
		w("%d. %s [%s, %s]\n   Why: %s\n   How: %s", q.N, q.Title, q.Effort, q.Impact, q.Why, q.How)
	}
	w("\n%s\n\n%s", v.SignOff, v.Footer)
	return b.String()
}
