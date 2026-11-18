package report

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/mailer"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/security"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/types"
)

// ErrAlreadyRunning is returned when a report is already being generated.
var ErrAlreadyRunning = errors.New("a report is already being generated")

// Service generates, stores and emails weekly reports.
type Service struct {
	App core.App
	Gen *Generator
	// Recipients is the default list. When empty, the report goes to the
	// instance's superusers.
	Recipients []string
	// PublicURL is the site's public origin (ASO_PUBLIC_URL), used to build
	// each report's permalink. When empty it falls back to PocketBase's
	// Application URL setting, unless that is still the localhost default.
	PublicURL string

	running atomic.Bool
}

// RunOptions control one run.
type RunOptions struct {
	// Send emails the report; false only generates and stores it.
	Send bool
	// To overrides the default recipients (useful for a test send).
	To []string
}

// Result summarises a run.
type Result struct {
	ID string `json:"id"`
	// Path is the report's permalink path; URL is the absolute link, empty
	// when no public URL is configured (in which case emails omit the link).
	Path       string   `json:"path"`
	URL        string   `json:"url"`
	Subject    string   `json:"subject"`
	Sent       bool     `json:"sent"`
	Recipients []string `json:"recipients"`
	Model      string   `json:"model"`
	Usage      Usage    `json:"usage"`
}

// Run generates a report, stores it, and optionally emails it. The report is
// stored before sending so a mail failure never loses the work.
func (s *Service) Run(ctx context.Context, opt RunOptions) (*Result, error) {
	if !s.running.CompareAndSwap(false, true) {
		return nil, ErrAlreadyRunning
	}
	defer s.running.Store(false)

	var to []mail.Address
	if opt.Send {
		var err error
		if to, err = s.recipients(opt.To); err != nil {
			return nil, err
		}
	}

	rep, err := s.Gen.Generate(ctx)
	if err != nil {
		return nil, err
	}
	slug := security.RandomStringWithAlphabet(24, slugAlphabet)
	rec, err := s.save(rep, to, slug)
	if err != nil {
		return nil, fmt.Errorf("store report: %w", err)
	}
	res := &Result{ID: rec.Id, Path: ViewPath(slug), URL: s.viewURL(slug), Subject: rep.Email.Subject, Model: rep.Model, Usage: rep.Usage}

	if opt.Send {
		// The stored (web) copy has no "view in browser" link; the emailed copy does.
		email := rep.Email
		if res.URL != "" {
			var err error
			meta := rep.Meta
			meta.WebURL = res.URL
			if email, err = Render(rep.Content, rep.Snapshot, meta); err != nil {
				return res, fmt.Errorf("render email: %w", err)
			}
		}
		if err := s.send(email, to); err != nil {
			return res, fmt.Errorf("report %s was saved but sending failed: %w", rec.Id, err)
		}
		rec.Set("sent_at", types.NowDateTime())
		if err := s.App.Save(rec); err != nil {
			return res, err
		}
		res.Sent = true
		for _, a := range to {
			res.Recipients = append(res.Recipients, a.Address)
		}
	}
	s.App.Logger().Info("weekly report", "id", rec.Id, "sent", res.Sent, "subject", res.Subject,
		"turns", rep.Usage.Turns, "input_tokens", rep.Usage.Input, "output_tokens", rep.Usage.Output)
	return res, nil
}

func (s *Service) recipients(override []string) ([]mail.Address, error) {
	list := override
	if len(list) == 0 {
		list = s.Recipients
	}
	var out []mail.Address
	for _, raw := range list {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		a, err := mail.ParseAddress(raw)
		if err != nil {
			return nil, fmt.Errorf("bad recipient %q: %w", raw, err)
		}
		out = append(out, *a)
	}
	if len(out) > 0 {
		return out, nil
	}
	supers, err := s.App.FindAllRecords(core.CollectionNameSuperusers)
	if err != nil {
		return nil, err
	}
	for _, r := range supers {
		out = append(out, mail.Address{Address: r.GetString("email")})
	}
	if len(out) == 0 {
		return nil, errors.New("no recipients: set ASO_REPORT_TO or create a superuser")
	}
	return out, nil
}

const slugAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// ViewPath is the public path a report is served at.
func ViewPath(slug string) string { return "/api/aso/report/view/" + slug }

func (s *Service) baseURL() string {
	base := strings.TrimSpace(s.PublicURL)
	if base == "" {
		base = s.App.Settings().Meta.AppURL
		// PocketBase's default Application URL is a localhost placeholder,
		// which would put a dead link in a real email.
		if u, err := url.Parse(base); err != nil || u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" {
			return ""
		}
	}
	return strings.TrimRight(base, "/")
}

func (s *Service) viewURL(slug string) string {
	if base := s.baseURL(); base != "" {
		return base + ViewPath(slug)
	}
	return ""
}

func (s *Service) save(rep *Report, to []mail.Address, slug string) (*core.Record, error) {
	col, err := s.App.FindCollectionByNameOrId("reports")
	if err != nil {
		return nil, err
	}
	var rcpt []string
	for _, a := range to {
		rcpt = append(rcpt, a.Address)
	}
	rec := core.NewRecord(col)
	rec.Set("slug", slug)
	rec.Set("subject", rep.Email.Subject)
	rec.Set("headline", rep.Content.Headline)
	rec.Set("model", rep.Model)
	rec.Set("content", rep.Content)
	rec.Set("usage", map[string]any{"turns": rep.Usage.Turns, "input_tokens": rep.Usage.Input,
		"output_tokens": rep.Usage.Output, "cache_read_tokens": rep.Usage.CacheRead, "cache_write_tokens": rep.Usage.CacheWrite})
	rec.Set("html", rep.Email.HTML)
	rec.Set("recipients", strings.Join(rcpt, ", "))
	return rec, s.App.Save(rec)
}

func (s *Service) send(e *Email, to []mail.Address) error {
	meta := s.App.Settings().Meta
	return s.App.NewMailClient().Send(&mailer.Message{
		From:    mail.Address{Name: meta.SenderName, Address: meta.SenderAddress},
		To:      to,
		Subject: e.Subject,
		HTML:    e.HTML,
		Text:    e.Text,
	})
}

// HTMLBySlug returns a stored report's page by its public slug.
func (s *Service) HTMLBySlug(slug string) (string, error) {
	// Reject anything that can't be a slug before touching the database.
	if len(slug) < 20 || len(slug) > 64 || strings.Trim(slug, slugAlphabet) != "" {
		return "", errNoReport
	}
	rec, err := s.App.FindFirstRecordByFilter("reports", "slug = {:slug}", map[string]any{"slug": slug})
	if err != nil {
		return "", errNoReport
	}
	return rec.GetString("html"), nil
}

var errNoReport = errors.New("report not found")

// LatestHTML returns the most recent stored report's email HTML.
func (s *Service) LatestHTML() (string, error) {
	recs, err := s.App.FindRecordsByFilter("reports", "id != ''", "-created", 1, 0)
	if err != nil || len(recs) == 0 {
		return "", errors.New("no report has been generated yet")
	}
	return recs[0].GetString("html"), nil
}

// Previous returns the last report that was actually emailed, so the next one
// can follow up on its advice. It returns nil when there is none.
func (s *Service) Previous(ctx context.Context) (*Previous, error) {
	recs, err := s.App.FindRecordsByFilter("reports", "sent_at != ''", "-sent_at", 1, 0)
	if err != nil || len(recs) == 0 {
		return nil, nil
	}
	rec := recs[0]
	var c Content
	if err := json.Unmarshal([]byte(rec.GetString("content")), &c); err != nil {
		return nil, nil
	}
	p := &Previous{SentAt: rec.GetDateTime("sent_at").Time().UTC().Truncate(time.Minute),
		Subject: rec.GetString("subject"), Headline: c.Headline, Intro: c.Intro}
	for _, q := range c.Quests {
		p.Quests = append(p.Quests, q.Title+": "+q.How)
	}
	return p, nil
}
