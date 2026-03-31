package migrations

import (
	"encoding/json"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/daos"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/models"
)

func init() {
	m.Register(func(db dbx.Builder) error {
		jsonData := `[
			{
				"id": "mg1gpqbe8lngni4",
				"created": "2024-03-27 10:28:25.072Z",
				"updated": "2024-03-27 10:42:01.365Z",
				"name": "steps",
				"type": "base",
				"system": false,
				"schema": [
					{
						"system": false,
						"id": "m4jjbahm",
						"name": "user",
						"type": "relation",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"collectionId": "_pb_users_auth_",
							"cascadeDelete": true,
							"minSelect": null,
							"maxSelect": 1,
							"displayFields": null
						}
					},
					{
						"system": false,
						"id": "bdsagnvp",
						"name": "value",
						"type": "number",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"noDecimal": false
						}
					},
					{
						"system": false,
						"id": "lcljrqug",
						"name": "date_from",
						"type": "date",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": "",
							"max": ""
						}
					},
					{
						"system": false,
						"id": "fqfziq07",
						"name": "date_to",
						"type": "date",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": "",
							"max": ""
						}
					},
					{
						"system": false,
						"id": "ke8hqccz",
						"name": "device_id",
						"type": "text",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"pattern": ""
						}
					},
					{
						"system": false,
						"id": "sjdwbffy",
						"name": "source_id",
						"type": "text",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"pattern": ""
						}
					},
					{
						"system": false,
						"id": "gezsstjx",
						"name": "source_name",
						"type": "text",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"pattern": ""
						}
					}
				],
				"indexes": [
					"CREATE INDEX ` + "`" + `idx_2xjM8jl` + "`" + ` ON ` + "`" + `steps` + "`" + ` (` + "`" + `user` + "`" + `)"
				],
				"listRule": "user = @request.auth.id",
				"viewRule": "user = @request.auth.id",
				"createRule": "",
				"updateRule": "user = @request.auth.id",
				"deleteRule": "user = @request.auth.id",
				"options": {}
			},
			{
				"id": "_pb_users_auth_",
				"created": "2024-03-27 10:31:41.712Z",
				"updated": "2024-03-27 10:51:41.944Z",
				"name": "users",
				"type": "auth",
				"system": false,
				"schema": [
					{
						"system": false,
						"id": "jih0aynb",
						"name": "event_date",
						"type": "date",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": "",
							"max": ""
						}
					}
				],
				"indexes": [],
				"listRule": "id = @request.auth.id",
				"viewRule": "id = @request.auth.id",
				"createRule": "",
				"updateRule": "id = @request.auth.id",
				"deleteRule": "id = @request.auth.id",
				"options": {
					"allowEmailAuth": true,
					"allowOAuth2Auth": true,
					"allowUsernameAuth": true,
					"exceptEmailDomains": null,
					"manageRule": null,
					"minPasswordLength": 8,
					"onlyEmailDomains": null,
					"onlyVerified": false,
					"requireEmail": false
				}
			},
			{
				"id": "r7s6uvdofnd6qd3",
				"created": "2024-03-27 10:44:47.750Z",
				"updated": "2024-03-27 10:44:59.607Z",
				"name": "walking_asymmetry_percentage",
				"type": "base",
				"system": false,
				"schema": [
					{
						"system": false,
						"id": "3ylt63ye",
						"name": "user",
						"type": "relation",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"collectionId": "_pb_users_auth_",
							"cascadeDelete": true,
							"minSelect": null,
							"maxSelect": 1,
							"displayFields": null
						}
					},
					{
						"system": false,
						"id": "k6zkzg2y",
						"name": "value",
						"type": "number",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"noDecimal": false
						}
					},
					{
						"system": false,
						"id": "g2hv0frp",
						"name": "date_from",
						"type": "date",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": "",
							"max": ""
						}
					},
					{
						"system": false,
						"id": "pmstcaeh",
						"name": "date_to",
						"type": "date",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": "",
							"max": ""
						}
					},
					{
						"system": false,
						"id": "zunmab1z",
						"name": "device_id",
						"type": "text",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"pattern": ""
						}
					},
					{
						"system": false,
						"id": "ou47bm61",
						"name": "source_id",
						"type": "text",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"pattern": ""
						}
					},
					{
						"system": false,
						"id": "st7kqjyc",
						"name": "source_name",
						"type": "text",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"pattern": ""
						}
					}
				],
				"indexes": [
					"CREATE INDEX ` + "`" + `idx_V9s5YVj` + "`" + ` ON ` + "`" + `walking_asymmetry_percentage` + "`" + ` (` + "`" + `user` + "`" + `)"
				],
				"listRule": "user = @request.auth.id",
				"viewRule": "user = @request.auth.id",
				"createRule": "",
				"updateRule": "user = @request.auth.id",
				"deleteRule": "user = @request.auth.id",
				"options": {}
			},
			{
				"id": "ygoovw0l3t5aoz2",
				"created": "2024-03-27 10:46:16.989Z",
				"updated": "2024-03-27 10:46:34.973Z",
				"name": "walking_double_support_percentage",
				"type": "base",
				"system": false,
				"schema": [
					{
						"system": false,
						"id": "lprjfwim",
						"name": "user",
						"type": "relation",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"collectionId": "_pb_users_auth_",
							"cascadeDelete": true,
							"minSelect": null,
							"maxSelect": 1,
							"displayFields": null
						}
					},
					{
						"system": false,
						"id": "1uxln32d",
						"name": "value",
						"type": "number",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"noDecimal": false
						}
					},
					{
						"system": false,
						"id": "n9vrhr1b",
						"name": "date_from",
						"type": "date",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": "",
							"max": ""
						}
					},
					{
						"system": false,
						"id": "xme9armn",
						"name": "date_to",
						"type": "date",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": "",
							"max": ""
						}
					},
					{
						"system": false,
						"id": "u1lpv3vx",
						"name": "device_id",
						"type": "text",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"pattern": ""
						}
					},
					{
						"system": false,
						"id": "mw5twuov",
						"name": "source_id",
						"type": "text",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"pattern": ""
						}
					},
					{
						"system": false,
						"id": "mgxg8xsn",
						"name": "source_name",
						"type": "text",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"pattern": ""
						}
					}
				],
				"indexes": [
					"CREATE INDEX ` + "`" + `idx_rQsRZOr` + "`" + ` ON ` + "`" + `walking_double_support_percentage` + "`" + ` (` + "`" + `user` + "`" + `)"
				],
				"listRule": "user = @request.auth.id",
				"viewRule": "user = @request.auth.id",
				"createRule": "",
				"updateRule": "user = @request.auth.id",
				"deleteRule": "user = @request.auth.id",
				"options": {}
			},
			{
				"id": "9mco9ft5r0nkpjv",
				"created": "2024-03-27 10:47:34.687Z",
				"updated": "2024-03-27 10:47:46.656Z",
				"name": "walking_speed",
				"type": "base",
				"system": false,
				"schema": [
					{
						"system": false,
						"id": "riceiwis",
						"name": "user",
						"type": "relation",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"collectionId": "_pb_users_auth_",
							"cascadeDelete": true,
							"minSelect": null,
							"maxSelect": 1,
							"displayFields": null
						}
					},
					{
						"system": false,
						"id": "wshk9247",
						"name": "value",
						"type": "number",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"noDecimal": false
						}
					},
					{
						"system": false,
						"id": "goiqnwqy",
						"name": "date_from",
						"type": "date",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": "",
							"max": ""
						}
					},
					{
						"system": false,
						"id": "d5yhsstp",
						"name": "date_to",
						"type": "date",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": "",
							"max": ""
						}
					},
					{
						"system": false,
						"id": "6vqzswoy",
						"name": "device_id",
						"type": "text",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"pattern": ""
						}
					},
					{
						"system": false,
						"id": "snxllheu",
						"name": "source_id",
						"type": "text",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"pattern": ""
						}
					},
					{
						"system": false,
						"id": "qhpv4dxk",
						"name": "source_name",
						"type": "text",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"pattern": ""
						}
					}
				],
				"indexes": [
					"CREATE INDEX ` + "`" + `idx_ixq7Irw` + "`" + ` ON ` + "`" + `walking_speed` + "`" + ` (` + "`" + `user` + "`" + `)"
				],
				"listRule": "user = @request.auth.id",
				"viewRule": "user = @request.auth.id",
				"createRule": "",
				"updateRule": "user = @request.auth.id",
				"deleteRule": "user = @request.auth.id",
				"options": {}
			},
			{
				"id": "hmz4hww9sjhkxh4",
				"created": "2024-03-27 10:48:34.481Z",
				"updated": "2024-03-27 10:48:44.626Z",
				"name": "walking_steadiness",
				"type": "base",
				"system": false,
				"schema": [
					{
						"system": false,
						"id": "n02aye2o",
						"name": "user",
						"type": "relation",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"collectionId": "_pb_users_auth_",
							"cascadeDelete": true,
							"minSelect": null,
							"maxSelect": 1,
							"displayFields": null
						}
					},
					{
						"system": false,
						"id": "6u62dehp",
						"name": "value",
						"type": "number",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"noDecimal": false
						}
					},
					{
						"system": false,
						"id": "6qtszo9h",
						"name": "date_from",
						"type": "date",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": "",
							"max": ""
						}
					},
					{
						"system": false,
						"id": "vc4irxv1",
						"name": "date_to",
						"type": "date",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": "",
							"max": ""
						}
					},
					{
						"system": false,
						"id": "hlkwt5oh",
						"name": "device_id",
						"type": "text",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"pattern": ""
						}
					},
					{
						"system": false,
						"id": "byszjney",
						"name": "source_id",
						"type": "text",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"pattern": ""
						}
					},
					{
						"system": false,
						"id": "thltw07g",
						"name": "source_name",
						"type": "text",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"pattern": ""
						}
					}
				],
				"indexes": [
					"CREATE INDEX ` + "`" + `idx_vcpdkhH` + "`" + ` ON ` + "`" + `walking_steadiness` + "`" + ` (` + "`" + `user` + "`" + `)"
				],
				"listRule": "user = @request.auth.id",
				"viewRule": "user = @request.auth.id",
				"createRule": "",
				"updateRule": "user = @request.auth.id",
				"deleteRule": "user = @request.auth.id",
				"options": {}
			},
			{
				"id": "abcx40ugfyygen6",
				"created": "2024-03-27 10:49:37.917Z",
				"updated": "2024-03-27 10:49:48.463Z",
				"name": "walking_step_length",
				"type": "base",
				"system": false,
				"schema": [
					{
						"system": false,
						"id": "fzzwkfwf",
						"name": "user",
						"type": "relation",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"collectionId": "_pb_users_auth_",
							"cascadeDelete": true,
							"minSelect": null,
							"maxSelect": 1,
							"displayFields": null
						}
					},
					{
						"system": false,
						"id": "3ysqzffx",
						"name": "value",
						"type": "number",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"noDecimal": false
						}
					},
					{
						"system": false,
						"id": "xilav59w",
						"name": "date_from",
						"type": "date",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": "",
							"max": ""
						}
					},
					{
						"system": false,
						"id": "zjjxww3i",
						"name": "date_to",
						"type": "date",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": "",
							"max": ""
						}
					},
					{
						"system": false,
						"id": "q16vns9w",
						"name": "device_id",
						"type": "text",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"pattern": ""
						}
					},
					{
						"system": false,
						"id": "h0hgq4vq",
						"name": "source_id",
						"type": "text",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"pattern": ""
						}
					},
					{
						"system": false,
						"id": "aneumsxl",
						"name": "source_name",
						"type": "text",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"pattern": ""
						}
					}
				],
				"indexes": [
					"CREATE INDEX ` + "`" + `idx_SJn4mwF` + "`" + ` ON ` + "`" + `walking_step_length` + "`" + ` (` + "`" + `user` + "`" + `)"
				],
				"listRule": "user = @request.auth.id",
				"viewRule": "user = @request.auth.id",
				"createRule": "",
				"updateRule": "user = @request.auth.id",
				"deleteRule": "user = @request.auth.id",
				"options": {}
			},
			{
				"id": "xgzlob39p1ooloe",
				"created": "2024-03-27 10:50:11.761Z",
				"updated": "2024-03-27 10:50:11.761Z",
				"name": "consent",
				"type": "base",
				"system": false,
				"schema": [
					{
						"system": false,
						"id": "3jufztft",
						"name": "field",
						"type": "relation",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"collectionId": "_pb_users_auth_",
							"cascadeDelete": true,
							"minSelect": null,
							"maxSelect": 1,
							"displayFields": null
						}
					},
					{
						"system": false,
						"id": "wsbsnpw7",
						"name": "consented",
						"type": "bool",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {}
					}
				],
				"indexes": [],
				"listRule": null,
				"viewRule": null,
				"createRule": null,
				"updateRule": null,
				"deleteRule": null,
				"options": {}
			},
			{
				"id": "r8x60e97o694ihv",
				"created": "2024-03-27 10:51:02.431Z",
				"updated": "2024-03-27 10:51:02.431Z",
				"name": "questionnaires",
				"type": "base",
				"system": false,
				"schema": [
					{
						"system": false,
						"id": "tmen8iab",
						"name": "user",
						"type": "relation",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"collectionId": "_pb_users_auth_",
							"cascadeDelete": true,
							"minSelect": null,
							"maxSelect": 1,
							"displayFields": null
						}
					},
					{
						"system": false,
						"id": "w50lrfuz",
						"name": "name",
						"type": "text",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"pattern": ""
						}
					},
					{
						"system": false,
						"id": "xtjmexnj",
						"name": "answers",
						"type": "json",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"maxSize": 2000000
						}
					}
				],
				"indexes": [
					"CREATE INDEX ` + "`" + `idx_KRbmwPl` + "`" + ` ON ` + "`" + `questionnaires` + "`" + ` (` + "`" + `user` + "`" + `)"
				],
				"listRule": null,
				"viewRule": null,
				"createRule": null,
				"updateRule": null,
				"deleteRule": null,
				"options": {}
			}
		]`

		collections := []*models.Collection{}
		if err := json.Unmarshal([]byte(jsonData), &collections); err != nil {
			return err
		}

		return daos.New(db).ImportCollections(collections, true, nil)
	}, func(db dbx.Builder) error {
		return nil
	})
}
