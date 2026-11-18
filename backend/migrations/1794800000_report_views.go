package migrations

import (
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/types"
)

// Lets the frontend render a report natively instead of showing server HTML.
//
//   - reports.view holds the fully resolved report (ranks, deltas, tiers,
//     daily ranks) as JSON, written by the backend when it generates a report.
//   - public_reports is a read-only VIEW over reports that exposes only what a
//     reader should see: id, slug, subject, created and view. Recipients,
//     token usage, model and the raw HTML are not selected, so they never
//     reach the public API.
//   - Its list rule only matches a record when the request carries that
//     record's slug (?slug=...). Without the slug nothing matches, so reports
//     can't be listed or enumerated, only fetched by someone who has the link.
func init() {
	m.Register(func(app core.App) error {
		reports, err := app.FindCollectionByNameOrId("reports")
		if err != nil {
			return err
		}
		reports.Fields.Add(&core.JSONField{Name: "view", MaxSize: 2 << 20})
		if err := app.Save(reports); err != nil {
			return err
		}

		v := core.NewViewCollection("public_reports")
		v.ViewQuery = "SELECT id, slug, subject, created, view FROM reports"
		v.ListRule = types.Pointer("slug = @request.query.slug")
		return app.Save(v)
	}, func(app core.App) error {
		if v, err := app.FindCollectionByNameOrId("public_reports"); err == nil {
			if err := app.Delete(v); err != nil {
				return err
			}
		}
		reports, err := app.FindCollectionByNameOrId("reports")
		if err != nil {
			return nil
		}
		reports.Fields.RemoveByName("view")
		return app.Save(reports)
	})
}
