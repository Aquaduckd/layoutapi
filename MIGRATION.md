# Migrate from file-based cmini

The catalog to copy is the **running bot’s** `layouts/` directory on the server. A laptop snapshot is not that tree.

This assumes both processes end up on the same machine. If the API is elsewhere, copy `layouts/` and use `https://…` for `LAYOUTAPI_URL`.

Paths below are examples. Substitute the real checkout locations.

## 1. Stop the old bot

That is what “pause add/remove” means. With the process down, Discord commands do nothing and `layouts/` cannot change.

If it is a systemd unit:

```sh
sudo systemctl stop cmini
```

Otherwise find and kill it:

```sh
ps aux | grep -E 'python3 main.py|poetry run python'
```

Stop that process (`Ctrl-C` in its terminal, or `kill` its PID). Confirm it is gone:

```sh
ps aux | grep '[m]ain.py'
```

Leave it stopped until step 6.

## 2. Copy the catalog

From the old cmini checkout:

```sh
ls layouts | wc -l
cp -a token.txt admins.py /tmp/cmini-backup/
cp -a layouts /tmp/cmini-backup/layouts
```

Then install this `layoutapi` tree on the host (clone or copy). Replace its `layouts/` with the live files:

```sh
rm -rf /path/to/layoutapi/layouts
cp -a /path/to/cmini/layouts /path/to/layoutapi/layouts
ls /path/to/layoutapi/layouts | wc -l
```

The two counts should match. Files with no `tag` / `blame` are treated as `cmini`; do not rewrite them first.

## 3. Create `apps.json`

In `/path/to/layoutapi` (this file is gitignored):

```sh
SECRET=$(python3 -c "import secrets; print(secrets.token_urlsafe(32))")
umask 077
cat > apps.json <<EOF
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
printf '%s\n' "$SECRET" > /path/to/cmini/layoutapi_token.txt
chmod 600 apps.json /path/to/cmini/layoutapi_token.txt
```

`apps.json` `secret` and `cmini/layoutapi_token.txt` must be the identical string. Do not commit either file.

## 4. Start layoutapi

Needs Go. From `/path/to/layoutapi`:

```sh
go run ./cmd/layoutapi
```

It should log that it loaded the layout count and `apps loaded: 1`, then listen on `:8080`. Leave it running.

Check:

```sh
curl -sS http://127.0.0.1:8080/health
curl -sS -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8080/v1/layouts/hours
```

Expect `{"ok":true,"count":…}` and `200`.

If anything other than this machine will **write**, put TLS in front (Caddy, nginx, Cloudflare) and use `https://…` in the next step. GET can stay public. The Go process itself can keep serving HTTP on localhost behind the proxy.

## 5. Configure the API-backed bot

The Discord bot must be the version that talks to layoutapi (`util/memory.py` uses HTTP, not a local `layouts/` folder). Deploy that code into `/path/to/cmini`, keeping the same `token.txt` if you want the same bot user.

From `/path/to/cmini`:

```sh
poetry env use python3.10
poetry install
```

If the bot and API are on the same host, the default URL is already `http://127.0.0.1:8080`. Otherwise:

```sh
export LAYOUTAPI_URL=https://your-layoutapi-host
```

`layoutapi_token.txt` from step 3 must be in this directory (the bot’s cwd when it starts). Discord privileged intents still required: **Message Content** and **Server Members**.

## 6. Start the new bot

The old process is still stopped from step 1. From `/path/to/cmini`:

```sh
poetry run python3 main.py
```

Do not start the old file-based `main.py` as well. One Discord token, one process.

## 7. Verify

In Discord, add a throwaway layout with `!cmini add`, then:

```sh
ls /path/to/layoutapi/layouts | grep -i throwaway-name
ls /path/to/cmini/layouts | grep -i throwaway-name
```

The new file must appear only under `layoutapi/layouts/`. Remove it with `!cmini remove` and confirm that file is gone from `layoutapi/layouts/`.

Corpora, cache, and Discord `token.txt` stay with the bot. After this, the old `cmini/layouts/` directory is leftover; nothing should write there.
