# layoutapi

Public catalog API. **GET is unauthenticated. Writes require a bearer token.**

`layouts/` is a snapshot copied from cmini.

## Run

```
cd layoutapi
go run ./cmd/layoutapi
```

Loads `tokens.txt` from the working directory if present.

Listens on `:8080`. Override with `-addr`, `-dir`, or `LAYOUTAPI_DIR`.

## Write tokens

Each application gets its own named token. Remove that line and restart to revoke it.

`tokens.txt` (not committed). dmini reads the same secret from `cmini/layoutapi_token.txt` (or `LAYOUTAPI_TOKEN`).

```
# name  secret
dmini   replace-with-a-long-random-string
web     another-long-random-string
```

Or environment:

```
# several named tokens
LAYOUTAPI_TOKENS=dmini:secret-one,web:secret-two

# one token (named "default")
LAYOUTAPI_TOKEN=secret-one
```

Writes send:

```
Authorization: Bearer <secret>
X-User-Id: <discord user id>
```

Each layout is stamped with the token name that created it (`source`). Later writes must use that same app token. `X-Admin: true` can bypass the Discord user check only within that app, not across apps. Existing files with no `source` are treated as `dmini`. Names are still globally unique.

If no tokens are configured, writes return 503.

## API

| Method | Path | Auth | Notes |
|---|---|---|---|
| `GET` | `/health` | public | count of loaded layouts |
| `GET` | `/v1/layouts` | public | `q`, `board`, `user`, `limit`, `offset`, `full=1` |
| `GET` | `/v1/layouts/{name}` | public | full layout JSON |
| `POST` | `/v1/layouts` | bearer | create; 409 if the name exists |
| `PUT` | `/v1/layouts/{name}` | bearer | replace; same app + owner only |
| `DELETE` | `/v1/layouts/{name}` | bearer | same app + owner only |
| `POST` | `/v1/layouts/{name}/rename` | bearer | body `{"name":"new-name"}`; same app + owner |
