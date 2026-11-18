package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/joho/godotenv"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/plugins/migratecmd"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/osutils"

	"aso-tracker/internal/itunes"
	"aso-tracker/internal/keywords"
	"aso-tracker/internal/report"
	"aso-tracker/internal/tracker"
	_ "aso-tracker/migrations"
)

func main() {
	// Optional backend/.env for local development. Variables already set in the
	// real environment (Docker, a host's secrets) take precedence over the file.
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("reading .env: %v", err)
	}

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

	// Weekly report: Claude reads the rank data through MCP tools and writes the
	// email. See internal/report.
	reportTools := &report.Tools{
		Data:     report.CacheDataset(func(context.Context) (*report.Dataset, error) { return report.Load(app, time.Now()) }, time.Minute),
		Listings: report.CacheListings(report.ITunesListings(runner.Client), 10*time.Minute),
	}
	reports := &report.Service{
		App:         app,
		Recipients:  splitList(os.Getenv("ASO_REPORT_TO")),
		PublicURL:   os.Getenv("ASO_PUBLIC_URL"),
		FrontendURL: os.Getenv("ASO_FRONTEND_URL"),
		Gen: &report.Generator{Agent: &report.Agent{
			Client: anthropic.NewClient(), // ANTHROPIC_API_KEY
			Model:  envString("ASO_REPORT_MODEL", report.DefaultModel),
			Tools:  reportTools,
			Log:    app.Logger().Info,
		}},
	}
	reportTools.Previous = reports.Previous
	haveAnthropicKey := os.Getenv("ANTHROPIC_API_KEY") != "" || os.Getenv("ANTHROPIC_AUTH_TOKEN") != ""

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

		// Weekly email, Mondays 14:00 UTC by default. ASO_REPORT_CRON=off disables it.
		reportCron := envString("ASO_REPORT_CRON", "0 14 * * 1")
		switch {
		case reportCron == "off":
			app.Logger().Info("weekly report disabled by ASO_REPORT_CRON=off")
		case !haveAnthropicKey:
			app.Logger().Warn("weekly report not scheduled: ANTHROPIC_API_KEY is not set")
		default:
			app.Cron().MustAdd("weeklyReport", reportCron, func() {
				if _, err := reports.Run(context.Background(), report.RunOptions{Send: true}); err != nil {
					app.Logger().Error("scheduled weekly report", "err", err)
				}
			})
		}

		// Generate (and by default email) a report now. Superuser only.
		//   POST /api/aso/report/run                       async, emails the default recipients
		//   POST /api/aso/report/run?send=false            generate and store only
		//   POST /api/aso/report/run?to=me@example.com     test send to one address
		//   POST /api/aso/report/run?wait=true             block and return the result (takes minutes)
		se.Router.POST("/api/aso/report/run", func(e *core.RequestEvent) error {
			if !haveAnthropicKey {
				return e.BadRequestError("ANTHROPIC_API_KEY is not set on the server", nil)
			}
			q := e.Request.URL.Query()
			opt := report.RunOptions{Send: q.Get("send") != "false", To: splitList(q.Get("to"))}
			if q.Get("wait") == "true" {
				// Detach from the caller's connection: if their client or proxy
				// times out, the run (and its API spend) should still finish.
				res, err := reports.Run(context.WithoutCancel(e.Request.Context()), opt)
				if errors.Is(err, report.ErrAlreadyRunning) {
					return e.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
				}
				if err != nil {
					return e.InternalServerError("report failed", err)
				}
				return e.JSON(http.StatusOK, res)
			}
			go func() {
				if _, err := reports.Run(context.Background(), opt); err != nil {
					app.Logger().Error("manual weekly report", "err", err)
				}
			}()
			return e.JSON(http.StatusAccepted, map[string]string{"status": "started"})
		}).Bind(apis.RequireSuperuserAuth())

		// The latest stored report as a web page (superuser only).
		se.Router.GET("/api/aso/report/latest", func(e *core.RequestEvent) error {
			html, err := reports.LatestHTML()
			if err != nil {
				return e.NotFoundError(err.Error(), nil)
			}
			return e.HTML(http.StatusOK, html)
		}).Bind(apis.RequireSuperuserAuth())

		// Each report's permalink. Public: the unguessable slug is the credential.
		// GET /api/aso/report/view/{slug}
		se.Router.GET("/api/aso/report/view/{slug}", func(e *core.RequestEvent) error {
			html, err := reports.HTMLBySlug(e.Request.PathValue("slug"))
			if err != nil {
				return e.NotFoundError("Report not found.", nil)
			}
			h := e.Response.Header()
			h.Set("X-Robots-Tag", "noindex, nofollow, noarchive")
			// Keep the slug out of Referer headers sent to the app-icon CDN.
			h.Set("Referrer-Policy", "no-referrer")
			h.Set("Cache-Control", "private, max-age=300")
			h.Set("Content-Security-Policy",
				"default-src 'none'; img-src https:; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
			return e.HTML(http.StatusOK, html)
		})

		// A design preview built from made-up data: no auth, no database, no API cost.
		se.Router.GET("/api/aso/report/sample", func(e *core.RequestEvent) error {
			snap, err := report.SampleDataset(report.SampleNow).Snapshot(report.SampleNow, report.Window)
			if err != nil {
				return err
			}
			mail, err := report.Render(report.SampleContent(), snap, report.Meta{})
			if err != nil {
				return err
			}
			return e.HTML(http.StatusOK, mail.HTML)
		})

		// The same read-only tools Claude uses for the report, over MCP, so you can
		// ask your own questions from Claude Desktop or Claude Code (superuser only).
		mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return reportTools.Server() }, nil)
		// Explicit methods: an any-method route would conflict with the static GET catch-all.
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
			se.Router.Route(method, "/api/aso/mcp", apis.WrapStdHandler(mcpHandler)).Bind(apis.RequireSuperuserAuth())
		}

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

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
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
