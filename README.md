# hayden

Hayden watches web pages and POSTs a webhook when one matches. It started as a PS5 stock checker.

## Run

```sh
docker run -p 8080:8080 \
  -e DATABASE_URL=postgres://user:pass@host:5432/hayden \
  -e HAYDEN_API_TOKEN=$(openssl rand -hex 32) \
  ghcr.io/icco/hayden:main
```

| Variable | Required | Meaning |
| --- | --- | --- |
| `DATABASE_URL` | yes | Postgres connection string. The schema migrates on startup. |
| `HAYDEN_API_TOKEN` | yes | Bearer token for every route except `/healthz` and `/metrics`. |
| `PORT` | no | Listen port, default `8080`. |

The default hook and scan period live in [`server/static/config.json`](server/static/config.json).

## Usage

```sh
export H=https://hayden.example.com AUTH="Authorization: Bearer $HAYDEN_API_TOKEN"

curl -H "$AUTH" -X POST $H/targets -d '{
  "name": "ps5",
  "url": "https://store.example.com/ps5",
  "fetch_mode": "headless",
  "match_type": "css",
  "match_value": "button.add-to-cart:not([disabled])"
}'
curl -H "$AUTH" $H/targets
curl -H "$AUTH" -X POST $H/force
curl -H "$AUTH" -X DELETE $H/targets/1
```

- `fetch_mode`: `http` (default) or `headless` for pages rendered by JavaScript.
- `match_type`: `substring` (default), `css`, `regex`, or `jsonpath` ([gjson syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md)).
- `invert`: match when the value is absent.
- `notify_mode`: `once` fires when a target starts matching, `change` fires whenever the page changes while it matches.
- `hook`, `period` (seconds): override the defaults per target.

The webhook body is JSON: `{"target", "url", "matched", "matched_at", "match_type"}`.

`GET /` (with the Bearer header) shows a status table, and `/metrics` serves Prometheus metrics. Package docs are on [pkg.go.dev](https://pkg.go.dev/go.icco.me/hayden).
