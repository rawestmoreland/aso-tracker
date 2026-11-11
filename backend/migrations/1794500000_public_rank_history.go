package migrations

import (
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// Open rank_history for public read so the keyword details page can chart it
// (latest_ranks is already public). Writes stay superuser-only.
func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("rank_history")
		if err != nil {
			return err
		}
		empty := ""
		collection.ListRule = &empty
		collection.ViewRule = &empty
		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("rank_history")
		if err != nil {
			return err
		}
		collection.ListRule = nil
		collection.ViewRule = nil
		return app.Save(collection)
	})
}
