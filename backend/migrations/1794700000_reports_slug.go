package migrations

import (
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/security"
)

const slugAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// Adds a random, unguessable slug to every report so each one can live at its
// own public URL (GET /api/aso/report/view/{slug}). 24 characters of
// [A-Za-z0-9] is about 143 bits, so the slug itself is the capability. The
// collection's API rules stay null: the page is served by a dedicated route.
func init() {
	m.Register(func(app core.App) error {
		c, err := app.FindCollectionByNameOrId("reports")
		if err != nil {
			return err
		}

		// New records get a slug automatically; existing ones are backfilled below.
		c.Fields.Add(&core.TextField{
			Name:                "slug",
			Required:            true,
			Min:                 20,
			Max:                 64,
			Pattern:             `^[A-Za-z0-9]+$`,
			AutogeneratePattern: `[A-Za-z0-9]{24}`,
		})
		if err := app.Save(c); err != nil {
			return err
		}

		// Backfill before adding the unique index: every existing row currently
		// holds '' and would violate it.
		var ids []string
		if err := app.DB().NewQuery("SELECT id FROM reports WHERE slug = ''").Column(&ids); err != nil {
			return err
		}
		for _, id := range ids {
			_, err := app.DB().NewQuery("UPDATE reports SET slug = {:slug} WHERE id = {:id}").
				Bind(map[string]any{"slug": security.RandomStringWithAlphabet(24, slugAlphabet), "id": id}).
				Execute()
			if err != nil {
				return err
			}
		}

		c, err = app.FindCollectionByNameOrId("reports")
		if err != nil {
			return err
		}
		c.AddIndex("idx_reports_slug", true, "slug", "")
		return app.Save(c)
	}, func(app core.App) error {
		c, err := app.FindCollectionByNameOrId("reports")
		if err != nil {
			return nil
		}
		c.RemoveIndex("idx_reports_slug")
		c.Fields.RemoveByName("slug")
		return app.Save(c)
	})
}
