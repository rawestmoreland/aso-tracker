package report

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func sampleEmail(t *testing.T) (*Email, *Snapshot, *Content) {
	t.Helper()
	d := SampleDataset(SampleNow)
	snap, err := d.Snapshot(SampleNow, 7)
	if err != nil {
		t.Fatal(err)
	}
	c := SampleContent()
	if err := c.Normalize(func(id string) bool { _, ok := snap.Facts[id]; return ok }); err != nil {
		t.Fatal(err)
	}
	e, err := Render(c, snap, Meta{})
	if err != nil {
		t.Fatal(err)
	}
	return e, snap, c
}

func TestRenderSample(t *testing.T) {
	e, snap, c := sampleEmail(t)

	// Numbers come from the data, not the model: "streaks" is #3 from #12.
	f := snap.Facts["kw03"]
	if f.Rank != 3 || f.PrevRank == nil || *f.PrevRank != 12 {
		t.Fatalf("sample data drifted: %+v", f)
	}
	for _, want := range []string{"The Weekly Climb", c.Headline[:20], "streaks", "#3", "▲ 9", "This week's quests", "Habitual"} {
		if !strings.Contains(e.HTML, want) {
			t.Errorf("HTML missing %q", want)
		}
	}
	if strings.Contains(e.HTML, "ZgotmplZ") {
		t.Error("html/template rejected an inline CSS value (ZgotmplZ)")
	}
	for _, want := range []string{"THE WEEKLY CLIMB", "QUESTS", "streaks (US)"} {
		if !strings.Contains(e.Text, want) {
			t.Errorf("text missing %q:\n%s", want, e.Text)
		}
	}

	// Optional: write the files for eyeballing in a browser.
	if dir := os.Getenv("REPORT_PREVIEW_DIR"); dir != "" {
		if err := os.WriteFile(dir+"/sample.html", []byte(e.HTML), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dir+"/sample.txt", []byte(e.Text), 0o644); err != nil {
			t.Fatal(err)
		}
		// The same resolved report as JSON, for developing the web page against.
		raw, err := ViewJSON(c, snap, Meta{})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dir+"/sample.view.json", raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRenderEscapesModelText(t *testing.T) {
	_, snap, c := sampleEmail(t)
	c.Headline = `<script>alert("x")</script> & friends`
	c.Quests[0].Why = `<img src=x onerror=alert(1)>`
	e, err := Render(c, snap, Meta{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(e.HTML, "<script>alert") || strings.Contains(e.HTML, "<img src=x") {
		t.Error("model text was not escaped")
	}
}

func TestNormalizeDropsUnknownKeywordsAndFixesEnums(t *testing.T) {
	c := &Content{
		Headline: "h", Intro: "i", Mood: "euphoric",
		Climbers: Section{Items: []Pick{{KeywordID: "real", Comment: "ok"}, {KeywordID: "made-up", Comment: "x"}}},
		Quests:   []Quest{{Title: "t", Effort: "huge", Impact: "", KeywordIDs: []string{"real", "nope"}}},
	}
	if err := c.Normalize(func(id string) bool { return id == "real" }); err != nil {
		t.Fatal(err)
	}
	if c.Mood != "steady" || c.Quests[0].Effort != "medium" || c.Quests[0].Impact != "medium" {
		t.Errorf("enums not repaired: %+v %+v", c.Mood, c.Quests[0])
	}
	if len(c.Climbers.Items) != 1 || len(c.Quests[0].KeywordIDs) != 1 {
		t.Errorf("unknown keyword ids survived: %+v", c)
	}
	if c.Subject != "h" {
		t.Errorf("subject should fall back to headline, got %q", c.Subject)
	}

	if err := (&Content{Headline: "h", Intro: "i"}).Normalize(func(string) bool { return true }); err == nil {
		t.Error("a report with no quests must be rejected")
	}
}

func TestNormalizeKeepsHeadersOnOneLine(t *testing.T) {
	c := SampleContent()
	c.Subject = "Great week\r\nBcc: attacker@example.com"
	if err := c.Normalize(func(string) bool { return true }); err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(c.Subject, "\r\n") || c.Subject != "Great week Bcc: attacker@example.com" {
		t.Errorf("subject = %q", c.Subject)
	}
}

func TestWebLinkOnlyWhenURLSet(t *testing.T) {
	_, snap, c := sampleEmail(t)
	plain, err := Render(c, snap, Meta{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain.HTML, "View it in your browser") || strings.Contains(plain.Text, "View in browser") {
		t.Error("the web copy must not link to itself")
	}

	const u = "https://aso.example.com/api/aso/report/view/abc123"
	linked, err := Render(c, snap, Meta{WebURL: u})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(linked.HTML, `href="`+u+`"`) || !strings.Contains(linked.Text, "View in browser: "+u) {
		t.Error("emailed copy is missing the permalink")
	}
}

func TestViewJSONContract(t *testing.T) {
	_, snap, c := sampleEmail(t)
	raw, err := ViewJSON(c, snap, Meta{IconURL: "https://example.com/i.png"})
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}

	if v["version"] != float64(1) || v["app_name"] != "Habitual" || v["icon_url"] != "https://example.com/i.png" {
		t.Errorf("header fields wrong: version=%v app=%v", v["version"], v["app_name"])
	}
	if mood := v["mood"].(map[string]any); mood["key"] != "climbing" || mood["label"] != "On the climb" {
		t.Errorf("mood = %v", mood)
	}
	if got := len(v["hero_series"].([]any)); got != SparkDays {
		t.Errorf("hero_series has %d points", got)
	}

	// No email-only presentation leaks into the contract.
	for _, banned := range []string{"#E8A317", "#1E7F4F", "Fg", "Bg", "RankColor", "Spark"} {
		if strings.Contains(string(raw), banned) {
			t.Errorf("JSON contains presentation detail %q", banned)
		}
	}

	// A keyword row carries semantic data: streaks went #12 -> #3, tier summit.
	hl := v["highlights"].([]any)
	var streaks map[string]any
	for _, h := range hl {
		if kw, ok := h.(map[string]any)["keyword"].(map[string]any); ok && kw["term"] == "streaks" {
			streaks = kw
		}
	}
	if streaks == nil {
		t.Fatal("streaks highlight has no keyword row")
	}
	if streaks["rank"] != float64(3) || streaks["rank_text"] != "#3" || streaks["tier"] != "summit" ||
		streaks["change"].(map[string]any)["kind"] != "up" || len(streaks["daily_ranks"].([]any)) != SparkDays {
		t.Errorf("streaks row = %v", streaks)
	}

	// Empty lists serialise as [] so the frontend never meets null.
	c.Highlights, c.Quests = nil, nil
	c.Climbers.Items, c.Fallers.Items, c.Rivals.Callouts = nil, nil, nil
	raw, _ = ViewJSON(c, snap, Meta{})
	for _, want := range []string{`"highlights":[]`, `"quests":[]`, `"callouts":[]`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("expected %s in %s", want, raw)
		}
	}
}
