package tracker

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"

	"aso-tracker/internal/itunes"
)

// Runner checks every tracked keyword once and records a rank row for every
// tracked app (yours and competitors) from the same search response.
type Runner struct {
	App     core.App
	Client  *itunes.Client
	Workers int

	running atomic.Bool
}

var ErrAlreadyRunning = errors.New("rank check already running")

type trackedApp struct {
	recordID string
	trackID  int64
}

func (r *Runner) Run(ctx context.Context) error {
	if !r.running.CompareAndSwap(false, true) {
		return ErrAlreadyRunning
	}
	defer r.running.Store(false)

	kws, err := r.App.FindAllRecords("keywords", dbx.HashExp{"tracked": true})
	if err != nil {
		return err
	}
	appRecords, err := r.App.FindAllRecords("tracked_apps")
	if err != nil {
		return err
	}

	var apps []trackedApp
	for _, rec := range appRecords {
		id, err := strconv.ParseInt(rec.GetString("track_id"), 10, 64)
		if err != nil {
			r.App.Logger().Warn("skipping app with bad track_id", "record", rec.Id)
			continue
		}
		apps = append(apps, trackedApp{recordID: rec.Id, trackID: id})
	}
	if len(apps) == 0 || len(kws) == 0 {
		r.App.Logger().Info("rank check: nothing to do", "keywords", len(kws), "apps", len(apps))
		return nil
	}

	workers := r.Workers
	if workers < 1 {
		workers = 1
	}

	jobs := make(chan *core.Record)
	var wg sync.WaitGroup
	var ok, failed atomic.Int64

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for kw := range jobs {
				if err := r.checkOne(ctx, kw, apps); err != nil {
					failed.Add(1)
					r.App.Logger().Warn("rank check failed",
						"term", kw.GetString("term"), "country", kw.GetString("country"), "err", err)
					continue
				}
				ok.Add(1)
			}
		}()
	}

feed:
	for _, kw := range kws {
		select {
		case jobs <- kw:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)
	wg.Wait()

	r.App.Logger().Info("rank check finished",
		"ok", ok.Load(), "failed", failed.Load(), "interval", r.Client.Limiter.Interval().String())
	return ctx.Err()
}

func (r *Runner) checkOne(ctx context.Context, kw *core.Record, apps []trackedApp) error {
	results, err := r.Client.Search(ctx, kw.GetString("term"), kw.GetString("country"))
	if err != nil {
		return err
	}

	col, err := r.App.FindCollectionByNameOrId("rank_history")
	if err != nil {
		return err
	}

	// One transaction per keyword keeps the SQLite write lock short.
	return r.App.RunInTransaction(func(tx core.App) error {
		for _, a := range apps {
			rec := core.NewRecord(col)
			rec.Set("keyword", kw.Id)
			rec.Set("app", a.recordID)
			rec.Set("rank", itunes.RankOf(results, a.trackID))
			if err := tx.Save(rec); err != nil {
				return err
			}
		}
		return nil
	})
}
