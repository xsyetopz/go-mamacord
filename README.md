# go-mamacord

A Discord bot + admin API (website/dashboard), written in Go.

Repo internal name: `mamacord` (env vars, IDs, `custom_id` prefixes).

## Quick Start (Local, Bot Only)

```bash
go run ./cmd/mamacord init
go run ./cmd/mamacord dev
```

That:

- reads `.env.dev` (only `.env.dev` / `.env.prod` are supported)
- creates/opens the SQLite DB at `SQLITE_PATH` (default `./data/mamacord.sqlite`)
- applies pending SQLite migrations automatically
- starts the Discord bot

If you want to edit `.env.dev` manually instead:

1. Copy `.env.dev.example` -> `.env.dev`
2. Set `DISCORD_TOKEN=...`
3. Optional: set `DISCORD_DEV_GUILD_ID=...` for faster command iteration

## Dashboard (Local)

The dashboard is served from the **admin API origin** (single-origin).

You run:

- Terminal A: `go run ./cmd/mamacord dev`
- Terminal B (optional HMR): `cd apps/dashboard && bun install && bun run dev`

You open in the browser:

- `http://127.0.0.1:8081/`

Do not open `http://127.0.0.1:5173/` directly. That is Vite, and it will break
cookies and API JSON parsing.

### Required Env Vars

Minimum:

- `DISCORD_TOKEN=...`
- `MAMACORD_ADMIN_ADDR=127.0.0.1:8081`

To enable Discord sign-in:

- `MAMACORD_DASHBOARD_CLIENT_ID=...`
- `MAMACORD_DASHBOARD_CLIENT_SECRET=...`

Sessions:

- Dev default: if `MAMACORD_DASHBOARD_SESSION_SECRET` is missing, a random one
  is generated at startup (sessions reset on restart).
- Stable: set `MAMACORD_DASHBOARD_SESSION_SECRET` to 32+ characters.

### One-Time Discord Portal Setup (Redirect URIs)

Discord Developer Portal -> your application -> OAuth2 -> Redirects:

- `http://127.0.0.1:8081/api/auth/callback` (login)
- `http://127.0.0.1:8081/api/install/callback` (bot install)

They must match exactly (scheme, host, port, path).

## Dashboard (Production)

Canonical public production topology:

- the dashboard is hosted as a static site (GitHub Pages or similar)
- the admin API is hosted on a separate origin (example: `api.` subdomain)
- the dashboard calls the admin API using `api_origin` from `apps/dashboard/public/config.json` (example: `{"api_origin":"https://api.example.com"}`)

Recommended domain shape:

- dashboard: `https://example.com`
- admin API: `https://api.example.com`

Why this is the main public deployment shape:

- static hosting is cheap and simple
- the frontend can be cached/CDN-served separately from the bot host
- same-origin is still simpler for local dev and single-box self-hosting

Raw `*.github.io` hosting is supported, but discouraged as the repo's main public
default. Prefer a custom domain if you want GitHub Pages to be the canonical
public dashboard.

Minimum prod env (when `MAMACORD_ADMIN_ADDR` is enabled):

- `MAMACORD_PROD_MODE=1`
- `MAMACORD_ALLOW_UNSIGNED_PLUGINS=0`
- `MAMACORD_DASHBOARD_CLIENT_ID=...`
- `MAMACORD_DASHBOARD_CLIENT_SECRET=...`
- `MAMACORD_DASHBOARD_SESSION_SECRET=...` (32+ chars)
- `MAMACORD_PUBLIC_DASHBOARD_ORIGIN=https://...`
- `MAMACORD_PUBLIC_API_ORIGIN=https://...`
- `MAMACORD_DASHBOARD_ALLOWED_ORIGINS=https://...` (must include the dashboard origin)

Plugin signing in prod:

- bundled plugins are already signed
- keep `config/trusted_keys.json` on the installed machine
- for your own signer and plugin signing flow, see `docs/reference.md#signing-prod`

## Common Problems (Fast Fixes)

- Dashboard says “Admin API not reachable”:
  - start `go run ./cmd/mamacord dev`
  - open `http://127.0.0.1:8081/` (not `:5173`)
- Dashboard error: `Unexpected token '<' ... is not valid JSON`:
  - you opened Vite directly; open `http://127.0.0.1:8081/` instead
- Discord says “invalid OAuth2 URL” when inviting the bot:
  - add `/api/install/callback` to OAuth2 Redirect URIs (see above)
- Bot exits with `4014 Disallowed intent(s)`:
  - Discord Developer Portal -> Bot -> Privileged Gateway Intents
  - enable `Server Members Intent` (mamacord requests Guild Members in gateway)

## Reference Docs

Longer docs live in:

- `docs/reference.md` (Docker, commands/modules, Lua plugins, signing, compatibility, release builds)
- `docs/sbc-hosting.md` (Raspberry Pi, Orange Pi, ODROID, plus separate runbooks for build-on-device, cross-build, first install, and updates)

## License

[MIT](LICENSE)
