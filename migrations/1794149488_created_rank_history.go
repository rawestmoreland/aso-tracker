package migrations

import (
	"encoding/json/v2"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		jsonData := `{
			"createRule": null,
			"deleteRule": null,
			"fields": [
				{
					"autogeneratePattern": "[a-z0-9]{15}",
					"help": "",
					"hidden": false,
					"id": "text3208210256",
					"max": 15,
					"min": 15,
					"name": "id",
					"pattern": "^[a-z0-9]+$",
					"presentable": false,
					"primaryKey": true,
					"required": true,
					"system": true,
					"type": "text"
				},
				{
					"cascadeDelete": true,
					"collectionId": "pbc_3927043399",
					"help": "",
					"hidden": false,
					"id": "relation1519612219",
					"maxSelect": 0,
					"minSelect": 0,
					"name": "keyword",
					"presentable": false,
					"required": true,
					"system": false,
					"type": "relation"
				},
				{
					"cascadeDelete": true,
					"collectionId": "pbc_3136645696",
					"help": "",
					"hidden": false,
					"id": "relation3379458255",
					"maxSelect": 0,
					"minSelect": 0,
					"name": "app",
					"presentable": false,
					"required": true,
					"system": false,
					"type": "relation"
				},
				{
					"help": "",
					"hidden": false,
					"id": "number2289690853",
					"max": null,
					"min": null,
					"name": "rank",
					"onlyInt": true,
					"presentable": false,
					"required": false,
					"system": false,
					"type": "number"
				},
				{
					"hidden": false,
					"id": "autodate2990389176",
					"name": "created",
					"onCreate": true,
					"onUpdate": false,
					"presentable": false,
					"system": false,
					"type": "autodate"
				},
				{
					"hidden": false,
					"id": "autodate3332085495",
					"name": "updated",
					"onCreate": true,
					"onUpdate": true,
					"presentable": false,
					"system": false,
					"type": "autodate"
				},
				{
					"hidden": false,
					"id": "autodate3615485344",
					"name": "checked_at",
					"onCreate": true,
					"onUpdate": false,
					"presentable": false,
					"system": false,
					"type": "autodate"
				}
			],
			"id": "pbc_1387180319",
			"indexes": [
				"CREATE INDEX ` + "`" + `idx_sobxfpyz81` + "`" + ` ON ` + "`" + `rank_history` + "`" + ` (\n  ` + "`" + `keyword` + "`" + `,\n  ` + "`" + `app` + "`" + `,\n  ` + "`" + `checked_at` + "`" + `\n)"
			],
			"listRule": null,
			"name": "rank_history",
			"system": false,
			"type": "base",
			"updateRule": null,
			"viewRule": null
		}`

		collection := &core.Collection{}
		if err := json.Unmarshal([]byte(jsonData), &collection); err != nil {
			return err
		}

		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("pbc_1387180319")
		if err != nil {
			return err
		}

		return app.Delete(collection)
	})
}
