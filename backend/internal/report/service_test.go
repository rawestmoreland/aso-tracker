package report

import (
	"net/mail"
	"strings"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/tests"

	_ "aso-tracker/migrations"
)

func TestPermalinks(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	svc := &Service{App: app, PublicURL: "https://aso.example.com/"}
	e, snap, c := func() (*Email, *Snapshot, *Content) { return sampleEmail(t) }()
	rep := &Report{Content: c, Snapshot: snap, Email: e}

	slug := "AbCdEfGhIjKlMnOpQrStUvWx"
	if _, err := svc.save(rep, []mail.Address{{Address: "a@example.com"}}, slug, []byte(`{"version":1}`)); err != nil {
		t.Fatalf("save: %v", err)
	}

	if got := svc.viewURL(slug); got != "https://aso.example.com/api/aso/report/view/"+slug {
		t.Errorf("viewURL = %q", got)
	}
	if html, err := svc.HTMLBySlug(slug); err != nil || !strings.Contains(html, "The Weekly Climb") {
		t.Errorf("lookup by slug: err=%v len=%d", err, len(html))
	}

	for _, bad := range []string{"", "short", strings.Repeat("a", 24), slug + "!", "../../etc/passwd" + slug, strings.Repeat("A", 65)} {
		if _, err := svc.HTMLBySlug(bad); err == nil {
			t.Errorf("slug %q must not resolve", bad)
		}
	}

	// A second report with the same slug violates the unique index.
	if _, err := svc.save(rep, nil, slug, []byte(`{}`)); err == nil {
		t.Error("duplicate slug must be rejected")
	}

	// New records without an explicit slug get one automatically.
	col, _ := app.FindCollectionByNameOrId("reports")
	if col.Fields.GetByName("slug") == nil {
		t.Error("slug field missing")
	}
}

func TestBaseURLIgnoresLocalhostDefault(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	svc := &Service{App: app}
	app.Settings().Meta.AppURL = "http://localhost:8090"
	if got := svc.viewURL("x"); got != "" {
		t.Errorf("localhost placeholder should yield no link, got %q", got)
	}
	app.Settings().Meta.AppURL = "https://aso.example.com"
	if got := svc.viewURL("x"); !strings.HasPrefix(got, "https://aso.example.com/api/aso/report/view/") {
		t.Errorf("AppURL fallback = %q", got)
	}
}

func TestViewURLPrefersFrontend(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	svc := &Service{App: app, PublicURL: "https://api.example.com", FrontendURL: "https://aso.example.com/"}
	if got := svc.viewURL("abc"); got != "https://aso.example.com/reports/abc" {
		t.Errorf("frontend link = %q", got)
	}
	svc.FrontendURL = ""
	if got := svc.viewURL("abc"); got != "https://api.example.com/api/aso/report/view/abc" {
		t.Errorf("backend fallback = %q", got)
	}
}
