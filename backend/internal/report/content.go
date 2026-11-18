package report

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Content is what Claude writes. It contains words and choices only: which
// keywords to feature and what to say about them. Ranks, deltas and charts are
// looked up from the data at render time, never taken from the model.
type Content struct {
	Subject        string `json:"subject"`
	Preheader      string `json:"preheader"`
	Mood           string `json:"mood"`
	Headline       string `json:"headline"`
	Intro          string `json:"intro"`
	ScoreboardNote string `json:"scoreboard_note"`

	Highlights []Highlight `json:"highlights"`
	Climbers   Section     `json:"climbers"`
	Fallers    Section     `json:"fallers"`
	Rivals     Corner      `json:"competitor_corner"`
	Quests     []Quest     `json:"quests"`
	SignOff    string      `json:"sign_off"`
}

// Highlight is a short story card ("Streaks cracked the top 3").
type Highlight struct {
	Kind      string `json:"kind"` // win | loss | surprise | milestone
	Title     string `json:"title"`
	Body      string `json:"body"`
	KeywordID string `json:"keyword_id"` // optional, "" when none
}

// Section introduces a list of movers.
type Section struct {
	Intro string `json:"intro"`
	Items []Pick `json:"items"`
}

// Pick features one keyword with a line of commentary.
type Pick struct {
	KeywordID string `json:"keyword_id"`
	Comment   string `json:"comment"`
}

// Corner is the competitor roundup.
type Corner struct {
	Title    string    `json:"title"`
	Body     string    `json:"body"`
	Callouts []Callout `json:"callouts"`
}

// Callout is one competitor observation.
type Callout struct {
	App       string `json:"app"`
	Note      string `json:"note"`
	KeywordID string `json:"keyword_id"` // optional
}

// Quest is one recommended action for the week.
type Quest struct {
	Title      string   `json:"title"`
	Why        string   `json:"why"`
	How        string   `json:"how"`
	Effort     string   `json:"effort"` // quick | medium | big
	Impact     string   `json:"impact"` // low | medium | high
	KeywordIDs []string `json:"keyword_ids"`
}

var (
	moods      = []string{"summit", "climbing", "steady", "slipping", "storm"}
	kinds      = []string{"win", "loss", "surprise", "milestone"}
	efforts    = []string{"quick", "medium", "big"}
	impacts    = []string{"low", "medium", "high"}
	stringType = map[string]any{"type": "string"}
)

func enum(values ...string) map[string]any {
	vs := make([]any, len(values))
	for i, v := range values {
		vs[i] = v
	}
	return map[string]any{"type": "string", "enum": vs}
}

// object builds a closed object schema in which every property is required,
// as structured outputs demand. Optional values are modelled as "".
func object(props map[string]any, order ...string) map[string]any {
	return map[string]any{"type": "object", "properties": props, "required": order, "additionalProperties": false}
}

func array(items map[string]any) map[string]any {
	return map[string]any{"type": "array", "items": items}
}

// Schema is the JSON Schema Claude's final answer must satisfy.
func Schema() map[string]any {
	pick := object(map[string]any{"keyword_id": stringType, "comment": stringType}, "keyword_id", "comment")
	section := object(map[string]any{"intro": stringType, "items": array(pick)}, "intro", "items")
	return object(map[string]any{
		"subject":         stringType,
		"preheader":       stringType,
		"mood":            enum(moods...),
		"headline":        stringType,
		"intro":           stringType,
		"scoreboard_note": stringType,
		"highlights": array(object(map[string]any{
			"kind": enum(kinds...), "title": stringType, "body": stringType, "keyword_id": stringType,
		}, "kind", "title", "body", "keyword_id")),
		"climbers": section,
		"fallers":  section,
		"competitor_corner": object(map[string]any{
			"title": stringType, "body": stringType,
			"callouts": array(object(map[string]any{"app": stringType, "note": stringType, "keyword_id": stringType},
				"app", "note", "keyword_id")),
		}, "title", "body", "callouts"),
		"quests": array(object(map[string]any{
			"title": stringType, "why": stringType, "how": stringType,
			"effort": enum(efforts...), "impact": enum(impacts...),
			"keyword_ids": array(stringType),
		}, "title", "why", "how", "effort", "impact", "keyword_ids")),
		"sign_off": stringType,
	}, "subject", "preheader", "mood", "headline", "intro", "scoreboard_note", "highlights",
		"climbers", "fallers", "competitor_corner", "quests", "sign_off")
}

// ParseContent decodes the model's JSON answer.
func ParseContent(raw string) (*Content, error) {
	raw = strings.TrimSpace(raw)
	var c Content
	dec := json.NewDecoder(strings.NewReader(raw))
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("report JSON: %w", err)
	}
	return &c, nil
}

// oneLine collapses all whitespace, newlines included, so a model-written
// value can never inject extra mail headers through the subject line.
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func clip(s string, max int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:max-1])) + "…"
}

func oneOf(v string, allowed []string, fallback string) string {
	for _, a := range allowed {
		if v == a {
			return v
		}
	}
	return fallback
}

// Normalize makes Content safe to render: it clamps lengths and list sizes,
// forces enums into range, and drops references to keywords that are not in
// the data. It fails only when the report would be empty.
func (c *Content) Normalize(known func(keywordID string) bool) error {
	c.Subject = clip(oneLine(c.Subject), 90)
	c.Preheader = clip(oneLine(c.Preheader), 140)
	c.Headline = clip(oneLine(c.Headline), 140)
	c.Intro = clip(c.Intro, 900)
	c.ScoreboardNote = clip(c.ScoreboardNote, 220)
	c.SignOff = clip(c.SignOff, 300)
	c.Mood = oneOf(c.Mood, moods, "steady")

	ref := func(id string) string {
		if id != "" && known(id) {
			return id
		}
		return ""
	}

	hs := c.Highlights[:0]
	for _, h := range c.Highlights {
		if strings.TrimSpace(h.Title) == "" {
			continue
		}
		hs = append(hs, Highlight{Kind: oneOf(h.Kind, kinds, "surprise"), Title: clip(h.Title, 90),
			Body: clip(h.Body, 420), KeywordID: ref(h.KeywordID)})
	}
	c.Highlights = capN(hs, 4)

	for _, s := range []*Section{&c.Climbers, &c.Fallers} {
		s.Intro = clip(s.Intro, 300)
		items := s.Items[:0]
		for _, p := range s.Items {
			if ref(p.KeywordID) == "" {
				continue
			}
			items = append(items, Pick{KeywordID: p.KeywordID, Comment: clip(p.Comment, 220)})
		}
		s.Items = capN(items, 5)
	}

	c.Rivals.Title = clip(c.Rivals.Title, 90)
	c.Rivals.Body = clip(c.Rivals.Body, 600)
	cs := c.Rivals.Callouts[:0]
	for _, k := range c.Rivals.Callouts {
		if strings.TrimSpace(k.App) == "" || strings.TrimSpace(k.Note) == "" {
			continue
		}
		cs = append(cs, Callout{App: clip(k.App, 60), Note: clip(k.Note, 260), KeywordID: ref(k.KeywordID)})
	}
	c.Rivals.Callouts = capN(cs, 4)

	qs := c.Quests[:0]
	for _, q := range c.Quests {
		if strings.TrimSpace(q.Title) == "" {
			continue
		}
		var ids []string
		for _, id := range q.KeywordIDs {
			if ref(id) != "" && len(ids) < 4 {
				ids = append(ids, id)
			}
		}
		qs = append(qs, Quest{Title: clip(q.Title, 100), Why: clip(q.Why, 380), How: clip(q.How, 480),
			Effort: oneOf(q.Effort, efforts, "medium"), Impact: oneOf(q.Impact, impacts, "medium"), KeywordIDs: ids})
	}
	c.Quests = capN(qs, 5)

	if c.Headline == "" || c.Intro == "" {
		return errors.New("report is missing a headline or intro")
	}
	if len(c.Quests) == 0 {
		return errors.New("report has no quests (recommended actions)")
	}
	if c.Subject == "" {
		c.Subject = c.Headline
	}
	return nil
}

func capN[T any](s []T, n int) []T {
	if len(s) > n {
		return s[:n]
	}
	return s
}
