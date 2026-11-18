package report

import (
	"bytes"
	_ "embed"
	"encoding/json"
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

// moodStyle is the per-mood look. The colours are for the email only; the
// JSON carries just the key, label and emoji and the web page picks its own.
type moodStyle struct {
	Key              string       `json:"key"`
	Label            string       `json:"label"`
	Emoji            string       `json:"emoji"`
	From, To, Accent template.CSS `json:"-"`
}

var moodStyles = map[string]moodStyle{
	"summit":   {"summit", "Summit week", "🏔️", "#F59E0B", "#DC4C1B", "#E8A317"},
	"climbing": {"climbing", "On the climb", "🧗", "#0F766E", "#2F9E6B", "#2F9E6B"},
	"steady":   {"steady", "Steady ground", "⛺", "#335F8A", "#5B8DB8", "#3B82C4"},
	"slipping": {"slipping", "Losing altitude", "🪨", "#9A5B13", "#C2410C", "#C2410C"},
	"storm":    {"storm", "Storm warning", "⛈️", "#1F2937", "#4B5563", "#B3261E"},
}

// Meta is non-ranking context for the email chrome.
type Meta struct {
	IconURL string // optional app icon
	// WebURL, when set, adds a "view in browser" link. Leave it empty for the
	// copy served at that URL, which would otherwise link to itself.
	WebURL string
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

// chip is a small status pill. Kind is one of up, down, flat, new, out, none.
type chip struct {
	Text   string       `json:"text"`
	Kind   string       `json:"kind"`
	Fg, Bg template.CSS `json:"-"`
}

func newChip(kind, text string) chip {
	c := chip{Text: text, Kind: kind, Fg: colFlat, Bg: colFlatBg}
	switch kind {
	case "up":
		c.Fg, c.Bg = colUp, colUpBg
	case "down", "out":
		c.Fg, c.Bg = colDown, colDownBg
	case "new":
		c.Fg, c.Bg = colNewFg, colNewBg
	}
	return c
}

type tile struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Sub   string `json:"sub"`
	Delta chip   `json:"delta"`
}

// rowView is one keyword: where you rank now and how it got there. Tier is
// summit (top 3), ridge (top 10), foothills (top 30) or base.
type rowView struct {
	Term      string       `json:"term"`
	Country   string       `json:"country"`
	Rank      string       `json:"rank_text"`
	RankNum   int          `json:"rank"` // 0 = not in the top 200
	Tier      string       `json:"tier"`
	RankColor template.CSS `json:"-"`
	Change    chip         `json:"change"`
	Detail    string       `json:"detail"`
	Spark     []bar        `json:"-"`
	Daily     []int        `json:"daily_ranks"` // oldest first; -1 = not tracked yet
	Comment   string       `json:"comment,omitempty"`
}

type highlightView struct {
	Kind   string       `json:"kind"`
	Emoji  string       `json:"emoji"`
	Title  string       `json:"title"`
	Body   string       `json:"body"`
	Border template.CSS `json:"-"`
	Row    *rowView     `json:"keyword,omitempty"`
}

type sectionView struct {
	Title string    `json:"title"`
	Intro string    `json:"intro"`
	Rows  []rowView `json:"rows"`
}

type calloutView struct {
	App  string   `json:"app"`
	Note string   `json:"note"`
	Row  *rowView `json:"keyword,omitempty"`
}

type questView struct {
	N         int    `json:"n"`
	Title     string `json:"title"`
	Why       string `json:"why"`
	How       string `json:"how"`
	Effort    string `json:"effort"` // display label
	Impact    string `json:"impact"` // display label
	EffortKey string `json:"effort_key"`
	ImpactKey string `json:"impact_key"`

	EffortFg, EffortBg, ImpactFg, ImpactBg template.CSS `json:"-"`
	Rows                                   []rowView    `json:"keywords"`
}

// viewVersion is bumped when the JSON shape changes incompatibly; the web page
// checks it before rendering.
const viewVersion = 1

// view is the fully resolved report. It is rendered to HTML for email and
// serialised to JSON (colours excluded) for the web page, so both show the
// same numbers and wording.
type view struct {
	Version    int             `json:"version"`
	Subject    string          `json:"subject"`
	Preheader  string          `json:"preheader"`
	Mood       moodStyle       `json:"mood"`
	Week       string          `json:"week"`
	AppName    string          `json:"app_name"`
	IconURL    string          `json:"icon_url"`
	WebURL     string          `json:"-"`
	Headline   string          `json:"headline"`
	Intro      string          `json:"intro"`
	ScoreNote  string          `json:"scoreboard_note"`
	Hero       []bar           `json:"-"`
	HeroSeries []float64       `json:"hero_series"` // daily visibility, oldest first
	Tiles      []tile          `json:"tiles"`
	Highlights []highlightView `json:"highlights"`
	Climbers   sectionView     `json:"climbers"`
	Fallers    sectionView     `json:"fallers"`
	RivalTitle string          `json:"rivals_title"`
	RivalBody  string          `json:"rivals_body"`
	Callouts   []calloutView   `json:"callouts"`
	Quests     []questView     `json:"quests"`
	SignOff    string          `json:"sign_off"`
	Footer     string          `json:"footer"`
}

func tierName(rank int) string {
	switch {
	case rank >= 1 && rank <= 3:
		return "summit"
	case rank >= 1 && rank <= 10:
		return "ridge"
	case rank >= 1 && rank <= 30:
		return "foothills"
	default:
		return "base"
	}
}

func tierColor(rank int) template.CSS {
	switch tierName(rank) {
	case "summit":
		return colSummit
	case "ridge":
		return colRidge
	case "foothills":
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
		return newChip("none", "first week")
	case change > 0:
		return newChip("up", fmt.Sprintf("▲ %d", change))
	case change < 0:
		return newChip("down", fmt.Sprintf("▼ %d", -change))
	default:
		return newChip("flat", "no change")
	}
}

func rowFor(f KeywordFact) rowView {
	r := rowView{Term: f.Term, Country: strings.ToUpper(f.Country), Rank: rankText(f.Rank), RankNum: f.Rank,
		Tier: tierName(f.Rank), RankColor: tierColor(f.Rank), Spark: sparkBars(f.DailyRanks), Daily: f.DailyRanks}
	prev := ""
	if f.PrevRank != nil {
		prev = rankText(*f.PrevRank)
	}
	switch f.Move {
	case "up":
		r.Change, r.Detail = newChip("up", fmt.Sprintf("▲ %d", f.Change)), "from "+prev
	case "down":
		r.Change, r.Detail = newChip("down", fmt.Sprintf("▼ %d", -f.Change)), "from "+prev
	case "entered":
		r.Change, r.Detail = newChip("new", "NEW"), "entered the top 200"
	case "dropped_out":
		r.Change, r.Detail = newChip("out", "OUT"), "was "+prev
	case "flat":
		r.Change, r.Detail = newChip("flat", "steady"), "unchanged"
	default:
		r.Change, r.Detail = newChip("new", "new"), "just started tracking"
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
		Version: viewVersion,
		Subject: c.Subject, Preheader: c.Preheader, Mood: mood, AppName: s.Mine.Name, IconURL: m.IconURL, WebURL: m.WebURL,
		Week:     fmt.Sprintf("%s – %s", s.AsOf.AddDate(0, 0, -s.Days).Format("Jan 2"), s.AsOf.Format("Jan 2, 2006")),
		Headline: c.Headline, Intro: c.Intro, ScoreNote: c.ScoreboardNote,
		Hero:       heroBars(me.DailyVisibility),
		HeroSeries: me.DailyVisibility,
		SignOff:    c.SignOff,
		Footer: fmt.Sprintf("Tracking %d keywords across %d competitors. Ranks come from live App Store search results.",
			me.Keywords, len(s.Rivals)),
	}

	vis := deltaChip(0, false)
	if me.VisibilityChange != nil {
		switch ch := *me.VisibilityChange; {
		case ch > 0:
			vis = newChip("up", fmt.Sprintf("▲ %.1f", ch))
		case ch < 0:
			vis = newChip("down", fmt.Sprintf("▼ %.1f", -ch))
		default:
			vis = newChip("flat", "no change")
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
		hv := highlightView{Kind: h.Kind, Emoji: emoji[h.Kind], Title: stripLeadingEmoji(h.Title), Body: h.Body, Border: border[h.Kind]}
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
			Effort: effort[q.Effort], Impact: impactLabel(q.Impact), EffortKey: q.Effort, ImpactKey: q.Impact}
		if qv.ImpactKey == "" {
			qv.ImpactKey = "medium"
		}
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

	// The JSON should say [] rather than null for empty lists.
	if v.Highlights == nil {
		v.Highlights = []highlightView{}
	}
	if v.Callouts == nil {
		v.Callouts = []calloutView{}
	}
	if v.Quests == nil {
		v.Quests = []questView{}
	}
	if v.Climbers.Rows == nil {
		v.Climbers.Rows = []rowView{}
	}
	if v.Fallers.Rows == nil {
		v.Fallers.Rows = []rowView{}
	}
	for i := range v.Quests {
		if v.Quests[i].Rows == nil {
			v.Quests[i].Rows = []rowView{}
		}
	}
	return v
}

// ViewJSON resolves a report into the JSON the web page renders. It is the
// same data the email is built from, minus the email-only colours.
func ViewJSON(c *Content, s *Snapshot, m Meta) ([]byte, error) {
	return json.Marshal(buildView(c, s, m))
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

	if v.WebURL != "" {
		w("View in browser: %s\n", v.WebURL)
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
