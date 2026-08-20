# layoutapi

A small HTTP API for the keyboard layout catalog. Layout JSON files are the source of truth; this service reads and writes them.

`layouts/` is a snapshot copied from cmini. Cmini itself is not modified.

## Run

```
cd layoutapi
go run ./cmd/layoutapi
```

Listens on `:8080` and loads `./layouts` (4123 layouts). Override with `-addr`, `-dir`, or `LAYOUTAPI_DIR`.

## API

Mutating requests need `X-User-Id: <discord id>`. Set `X-Admin: true` to bypass ownership.

| Method | Path | Notes |
|---|---|---|
| `GET` | `/health` | count of loaded layouts |
| `GET` | `/v1/layouts` | summaries. Query: `q`, `board`, `user`, `limit`, `offset`, `full=1` |
| `GET` | `/v1/layouts/{name}` | full cmini JSON document |
| `POST` | `/v1/layouts` | create; 409 if the name exists |
| `PUT` | `/v1/layouts/{name}` | replace; owner only |
| `DELETE` | `/v1/layouts/{name}` | owner only |
| `POST` | `/v1/layouts/{name}/rename` | body `{"name":"new-name"}` |

Each file is the cmini shape: `{name, user, board, keys}` plus optional `free`.
