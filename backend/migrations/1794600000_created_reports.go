package migrations

import (
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// reports stores every generated weekly report: the rendered email, the
// structured content Claude wrote, and token usage. API rules are left null,
// so only superusers can read it.
func init() {
	m.Register(func(app core.App) error {
		c := core.NewBaseCollection("reports")
		c.Fields.Add(
			&core.TextField{Name: "subject", Required: true},
			&core.TextField{Name: "headline"},
			&core.TextField{Name: "model"},
			&core.JSONField{Name: "content", MaxSize: 2 << 20},
			&core.JSONField{Name: "usage", MaxSize: 1 << 16},
			&core.EditorField{Name: "html", MaxSize: 4 << 20},
			&core.TextField{Name: "recipients"},
			&core.DateField{Name: "sent_at"},
			&core.AutodateField{Name: "created", OnCreate: true},
		)
		c.AddIndex("idx_reports_created", false, "created", "")
		return app.Save(c)
	}, func(app core.App) error {
		c, err := app.FindCollectionByNameOrId("reports")
		if err != nil {
			return nil // already gone
		}
		return app.Delete(c)
	})
}
