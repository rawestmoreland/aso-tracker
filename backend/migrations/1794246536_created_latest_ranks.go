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
					"autogeneratePattern": "",
					"help": "",
					"hidden": false,
					"id": "text3208210256",
					"max": 0,
					"min": 0,
					"name": "id",
					"pattern": "^[a-z0-9]+$",
					"presentable": false,
					"primaryKey": true,
					"required": true,
					"system": true,
					"type": "text"
				},
				{
					"help": "",
					"hidden": false,
					"id": "json1519612219",
					"maxSize": 1,
					"name": "keyword",
					"presentable": false,
					"required": false,
					"system": false,
					"type": "json"
				},
				{
					"autogeneratePattern": "",
					"help": "",
					"hidden": false,
					"id": "_clone_S2Uc",
					"max": 0,
					"min": 0,
					"name": "term",
					"pattern": "",
					"presentable": true,
					"primaryKey": false,
					"required": true,
					"system": false,
					"type": "text"
				},
				{
					"autogeneratePattern": "",
					"help": "",
					"hidden": false,
					"id": "_clone_5qxS",
					"max": 0,
					"min": 0,
					"name": "country",
					"pattern": "",
					"presentable": false,
					"primaryKey": false,
					"required": true,
					"system": false,
					"type": "text"
				},
				{
					"help": "",
					"hidden": false,
					"id": "json3379458255",
					"maxSize": 1,
					"name": "app",
					"presentable": false,
					"required": false,
					"system": false,
					"type": "json"
				},
				{
					"autogeneratePattern": "",
					"help": "",
					"hidden": false,
					"id": "_clone_Q9bU",
					"max": 0,
					"min": 0,
					"name": "app_name",
					"pattern": "",
					"presentable": false,
					"primaryKey": false,
					"required": false,
					"system": false,
					"type": "text"
				},
				{
					"help": "",
					"hidden": false,
					"id": "_clone_PjBD",
					"name": "is_mine",
					"presentable": false,
					"required": false,
					"system": false,
					"type": "bool"
				},
				{
					"help": "",
					"hidden": false,
					"id": "json2289690853",
					"maxSize": 1,
					"name": "rank",
					"presentable": false,
					"required": false,
					"system": false,
					"type": "json"
				},
				{
					"help": "",
					"hidden": false,
					"id": "json1378752341",
					"maxSize": 1,
					"name": "rank_7d",
					"presentable": false,
					"required": false,
					"system": false,
					"type": "json"
				},
				{
					"help": "",
					"hidden": false,
					"id": "json3212157386",
					"maxSize": 1,
					"name": "rank_first",
					"presentable": false,
					"required": false,
					"system": false,
					"type": "json"
				},
				{
					"help": "",
					"hidden": false,
					"id": "json3615485344",
					"maxSize": 1,
					"name": "checked_at",
					"presentable": false,
					"required": false,
					"system": false,
					"type": "json"
				}
			],
			"id": "pbc_71917220",
			"indexes": [],
			"listRule": null,
			"name": "latest_ranks",
			"system": false,
			"type": "view",
			"updateRule": null,
			"viewQuery": "WITH latest AS (\n  SELECT keyword, app, rank, checked_at,\n         ROW_NUMBER() OVER (PARTITION BY keyword, app ORDER BY checked_at DESC) AS rn\n  FROM rank_history\n),\nweek_ago AS (\n  SELECT keyword, app, rank,\n         ROW_NUMBER() OVER (PARTITION BY keyword, app ORDER BY checked_at DESC) AS rn\n  FROM rank_history\n  WHERE checked_at <= datetime('now', '-7 days')\n),\nfirst_seen AS (\n  SELECT keyword, app, rank,\n         ROW_NUMBER() OVER (PARTITION BY keyword, app ORDER BY checked_at ASC) AS rn\n  FROM rank_history\n)\nSELECT\n  (l.keyword || '_' || l.app) AS id,\n  l.keyword    AS keyword,\n  k.term       AS term,\n  k.country    AS country,\n  l.app        AS app,\n  a.name       AS app_name,\n  a.is_mine    AS is_mine,\n  l.rank       AS rank,\n  w.rank       AS rank_7d,\n  f.rank       AS rank_first,\n  l.checked_at AS checked_at\nFROM latest l\nJOIN keywords k       ON k.id = l.keyword\nJOIN tracked_apps a   ON a.id = l.app\nLEFT JOIN week_ago w   ON w.keyword = l.keyword AND w.app = l.app AND w.rn = 1\nLEFT JOIN first_seen f ON f.keyword = l.keyword AND f.app = l.app AND f.rn = 1\nWHERE l.rn = 1",
			"viewRule": null
		}`

		collection := &core.Collection{}
		if err := json.Unmarshal([]byte(jsonData), &collection); err != nil {
			return err
		}

		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("pbc_71917220")
		if err != nil {
			return err
		}

		return app.Delete(collection)
	})
}
