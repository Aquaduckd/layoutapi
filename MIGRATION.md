# Migrate from file-based cmini

The live catalog is whatever is in the **running bot’s** `layouts/` directory, not a laptop snapshot. Copy that tree.

1. **Backup** the server’s `cmini/layouts/`, plus `token.txt` and `admins.py`. Pause `!cmini add` / `remove` if you cannot lose writes during the cutover.

2. **Install layoutapi** on the host (or a second host). Put the copied JSON files in `layoutapi/layouts/`. Files with no `tag` / `blame` are treated as `cmini`; you do not have to rewrite them first.

3. **Create `apps.json`** (gitignored). Give the Discord bot its own app that stamps the `cmini` tag:

```json
{
  "apps": [
    {
      "name": "cmini",
      "secret": "a-long-random-string",
      "tag": "cmini"
    }
  ]
}
```

4. **Run the API.** `go run ./cmd/layoutapi` (or a binary) from `layoutapi/`. Put TLS in front if anything besides localhost will write. Check `GET /health` and `GET /v1/layouts/hours`.

5. **Point the bot at the API.** Deploy the API-backed cmini (no local `layouts/` writes). Keep the same Discord `token.txt` if you want the same bot user. Set:

   - `LAYOUTAPI_URL` — `http://127.0.0.1:8080` if the bot and API share a machine, otherwise `https://…`
   - `cmini/layoutapi_token.txt` or `LAYOUTAPI_TOKEN` — the **same** secret as `apps.json`

   Use Python 3.10 (`poetry env use python3.10 && poetry install`). Discord Message Content and Server Members intents stay required.

6. **Cut over.** Stop the old file-based process, then start `poetry run python3 main.py`. Do not run both with the same bot token. After that, add/remove a throwaway layout and confirm a new file appears under `layoutapi/layouts/`, not the old `cmini/layouts/`.

Corpora, cache, and Discord auth stay with the bot. Only the catalog moves. The old `layouts/` directory is leftover and must not be written to after cutover.
