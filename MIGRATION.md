# Migrate from file-based cmini

The catalog to copy is the **running bot’s** `layouts/` directory on the server. A laptop snapshot is not that tree.

This assumes both processes end up on the same machine. If the API is elsewhere, copy `layouts/` and use `https://…` for `LAYOUTAPI_URL`.

Set the two checkouts once, then use those variables in every command:

```sh
CMINI=$HOME/cmini
LAYOUTAPI=$HOME/layoutapi
```

Change the assignments if the bot or API lives somewhere else (`pwd` in each repo).

## 1. Stop the old bot

Stop the running `main.py` process (`Ctrl-C`, `kill`, or `sudo systemctl stop cmini`). Leave it stopped until step 7 so `$CMINI/layouts` cannot change while you copy it.

## 2. Copy the catalog

```sh
mkdir -p /tmp/cmini-backup
ls "$CMINI/layouts" | wc -l
cp -a "$CMINI/layouts" /tmp/cmini-backup/layouts
```

Install this `layoutapi` tree at `$LAYOUTAPI` if it is not there yet (clone or copy). Replace its `layouts/` with the live files:

```sh
rm -rf "$LAYOUTAPI/layouts"
cp -a "$CMINI/layouts" "$LAYOUTAPI/layouts"
ls "$LAYOUTAPI/layouts" | wc -l
```

The two counts should match. Files with no `tag` / `blame` are treated as `cmini`; do not rewrite them first.

## 3. Create `apps.json`

`$LAYOUTAPI/apps.json` is gitignored.

```sh
SECRET=$(python3 -c "import secrets; print(secrets.token_urlsafe(32))")
umask 077
cat > "$LAYOUTAPI/apps.json" <<EOF
{
  "apps": [
    {
      "name": "cmini",
      "secret": "$SECRET",
      "tag": "cmini"
    }
  ]
}
EOF
printf '%s\n' "$SECRET" > "$CMINI/layoutapi_token.txt"
chmod 600 "$LAYOUTAPI/apps.json" "$CMINI/layoutapi_token.txt"
```

The `secret` in `apps.json` and the contents of `$CMINI/layoutapi_token.txt` must be the identical string. Do not commit either file.

## 4. Start layoutapi

Needs Go.

```sh
cd "$LAYOUTAPI"
go run ./cmd/layoutapi
```

It should log that it loaded the layout count and `apps loaded: 1`, then listen on `:8080`. Leave it running.

Check:

```sh
curl -sS http://127.0.0.1:8080/health
curl -sS -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8080/v1/layouts/hours
```

Expect `{"ok":true,"count":…}` and `200`.

If anything other than this machine will **write**, put TLS in front (Caddy, nginx, Cloudflare) and use `https://…` in step 6. GET can stay public. The Go process itself can keep serving HTTP on localhost behind the proxy.

## 5. Update cmini’s source

With the bot still stopped, replace the old file-based tree with the API-backed tree (`util/memory.py` talks HTTP, there is no write path to a local `layouts/` folder).

Keep `$CMINI/token.txt`, `$CMINI/admins.py`, and `$CMINI/layoutapi_token.txt`. Those are gitignored and will not come from git.

```sh
cd "$CMINI"
git fetch
git checkout <api-backed-branch>
```

Do this **after** step 2. The live `layouts/` must be copied out before the old tree is overwritten.

## 6. Install and point the bot at the API

```sh
cd "$CMINI"
poetry env use python3.10
poetry install
```

If the bot and API are on the same host, the default URL is already `http://127.0.0.1:8080`. Otherwise:

```sh
export LAYOUTAPI_URL=https://your-layoutapi-host
```

The bot must be started with cwd `$CMINI` so it finds `layoutapi_token.txt`. Discord privileged intents still required: **Message Content** and **Server Members**.

## 7. Start the new bot

The old process is still stopped from step 1.

```sh
cd "$CMINI"
poetry run python3 main.py
```

Do not start the old file-based `main.py` as well. One Discord token, one process.

## 8. Verify

In Discord, add a throwaway layout with `!cmini add`, then:

```sh
ls "$LAYOUTAPI/layouts" | grep -i throwaway-name
ls "$CMINI/layouts" | grep -i throwaway-name
```

The new file must appear only under `$LAYOUTAPI/layouts`. Remove it with `!cmini remove` and confirm that file is gone from `$LAYOUTAPI/layouts`.

Corpora, cache, and Discord `token.txt` stay with the bot. After this, `$CMINI/layouts` is leftover; nothing should write there.
