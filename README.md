# layoutapi

Public catalog API. **GET is unauthenticated. Writes require an app bearer token.**

`layouts/` is a snapshot copied from cmini.

## Run

```
cd layoutapi
go run ./cmd/layoutapi
```

Loads `apps.json` from the working directory if present.

Listens on `:8080`. Override with `-addr`, `-dir`, or `LAYOUTAPI_DIR`.

## Apps

Each client is an app with its own secret. A layout is stamped with a **tag** (`source` in the JSON), which is not necessarily the app name. Several apps can write the same tag.

`apps.json` (not committed):

```json
{
  "apps": [
    {
      "name": "dmini",
      "secret": "replace-with-a-long-random-string",
      "tag": "cmini"
    },
    {
      "name": "cmini",
      "secret": "another-long-random-string",
      "tag": "cmini"
    },
    {
      "name": "web",
      "secret": "a-third-long-random-string",
      "tag": "web"
    }
  ]
}
```

- `name` — app id, for revoke and logs
- `secret` — bearer token
- `tag` — stamped on create; defaults to `name`
- `write` — tags this app may mutate; defaults to `[tag]`

dmini reads its secret from `cmini/layoutapi_token.txt` (or `LAYOUTAPI_TOKEN`).

Writes send:

```
Authorization: Bearer <secret>
```

Discord user/admin checks belong to the client. The `user` field is metadata. Existing files with no `source` are treated as `cmini`. Names are globally unique. GET stays public.

If no apps are configured, writes return 503.

## API

| Method | Path | Auth | Notes |
|---|---|---|---|
| `GET` | `/health` | public | count of loaded layouts |
| `GET` | `/v1/layouts` | public | `q`, `board`, `user`, `limit`, `offset`, `full=1` |
| `GET` | `/v1/layouts/{name}` | public | full layout JSON |
| `POST` | `/v1/layouts` | bearer | create; stamps `tag`; 409 if the name exists |
| `PUT` | `/v1/layouts/{name}` | bearer | replace; app must be allowed to write that tag |
| `DELETE` | `/v1/layouts/{name}` | bearer | same |
| `POST` | `/v1/layouts/{name}/rename` | bearer | body `{"name":"new-name"}` |
