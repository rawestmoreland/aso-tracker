package report

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"sync/atomic"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/mailer"
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
	ID         string   `json:"id"`
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
	rec, err := s.save(rep, to)
	if err != nil {
		return nil, fmt.Errorf("store report: %w", err)
	}
	res := &Result{ID: rec.Id, Subject: rep.Email.Subject, Model: rep.Model, Usage: rep.Usage}

	if opt.Send {
		if err := s.send(rep.Email, to); err != nil {
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

func (s *Service) save(rep *Report, to []mail.Address) (*core.Record, error) {
	col, err := s.App.FindCollectionByNameOrId("reports")
	if err != nil {
		return nil, err
	}
	var rcpt []string
	for _, a := range to {
		rcpt = append(rcpt, a.Address)
	}
	rec := core.NewRecord(col)
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
