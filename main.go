package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/plugins/migratecmd"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/osutils"

	"aso-tracker/internal/itunes"
	"aso-tracker/internal/keywords"
	"aso-tracker/internal/tracker"
	_ "aso-tracker/migrations"
)

func main() {
	app := pocketbase.New()

	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		// enable auto creation of migration files when making collection changes in the Dashboard
		// (the IsProbablyGoRun check is to enable it only during development)
		Automigrate: osutils.IsProbablyGoRun(),
	})

	interval := envDuration("ASO_INTERVAL", 3*time.Second) // ~20 requests/min; tune after running cmd/probe
	runner := &tracker.Runner{
		App:     app,
		Client:  itunes.NewClient(itunes.NewLimiter(interval)),
		Workers: envInt("ASO_WORKERS", 3),
	}

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		// serves static files from the provided public dir (if exists)
		se.Router.GET("/{path...}", apis.Static(os.DirFS("./pb_public"), false))

		// Daily run. Cron expressions are evaluated in UTC by default.
		cronExpr := envString("ASO_CRON", "0 2 * * *")
		app.Cron().MustAdd("rankCheck", cronExpr, func() {
			if err := runner.Run(context.Background()); err != nil {
				app.Logger().Error("scheduled rank check", "err", err)
			}
		})

		// Trigger a run on demand (superuser only). Returns immediately.
		se.Router.POST("/api/aso/check-now", func(e *core.RequestEvent) error {
			go func() {
				if err := runner.Run(context.Background()); err != nil {
					app.Logger().Error("manual rank check", "err", err)
				}
			}()
			return e.JSON(http.StatusAccepted, map[string]string{"status": "started"})
		}).Bind(apis.RequireSuperuserAuth())

		// Bulk-load keywords: POST a keywords file as the request body.
		//   curl -X POST -H "Authorization: <superuser token>" \
		//        --data-binary @keywords.txt "http://127.0.0.1:8090/api/aso/import-keywords?country=us"
		se.Router.POST("/api/aso/import-keywords", func(e *core.RequestEvent) error {
			country := e.Request.URL.Query().Get("country")
			if country == "" {
				country = "us"
			}
			terms, err := keywords.Parse(e.Request.Body)
			if err != nil {
				return e.BadRequestError("could not read keywords", err)
			}

			col, err := app.FindCollectionByNameOrId("keywords")
			if err != nil {
				return err
			}
			added := 0
			for _, term := range terms {
				_, err := app.FindFirstRecordByFilter("keywords",
					"term = {:term} && country = {:country}",
					dbx.Params{"term": term, "country": country})
				if err == nil {
					continue // already present
				}
				if !errors.Is(err, sql.ErrNoRows) {
					return err
				}
				rec := core.NewRecord(col)
				rec.Set("term", term)
				rec.Set("country", country)
				rec.Set("tracked", true)
				if err := app.Save(rec); err != nil {
					return err
				}
				added++
			}
			return e.JSON(http.StatusOK, map[string]int{"added": added, "total": len(terms)})
		}).Bind(apis.RequireSuperuserAuth())

		return se.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

func envString(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil && v > 0 {
		return v
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(os.Getenv(key)); err == nil && v > 0 {
		return v
	}
	return def
}
