# go-mamacord

go-mamacord is a Discord bot with a web dashboard and admin API. It stores its data in PostgreSQL and applies pending database migrations when it starts.

## Before you start

You need:

- Go 1.26.6
- PostgreSQL (the included Docker service uses PostgreSQL 18)
- a Discord application and bot token
- Bun 1.3.14 only if you want to run or build the dashboard

In the Discord Developer Portal, open your application and enable **Server Members Intent** under **Bot → Privileged Gateway Intents**. The bot cannot connect without this intent.

## Run the bot locally

1. Start PostgreSQL. The included service uses the default development database settings:

   ```bash
   docker compose up -d postgres
   ```

2. Create `.env.dev`:

   ```bash
   go run ./cmd/mamacord init
   ```

3. Open `.env.dev` and set `DISCORD_TOKEN` to your bot token.

4. Start go-mamacord:

   ```bash
   go run ./cmd/mamacord dev
   ```

The `init` command creates the local PostgreSQL connection settings and a dashboard session secret. The `dev` command reads `.env.dev`, starts the bot and admin API, and defaults to `http://127.0.0.1:8081` for the admin API.

To create the file by hand instead, copy [`.env.dev.example`](.env.dev.example) to `.env.dev`. Only `.env.dev` and `.env.prod` are supported.

Set `DISCORD_DEV_GUILD_ID` in `.env.dev` if you want Discord commands to update in one server while you develop.

## Open the local dashboard

Keep the bot running, then start the dashboard in another terminal:

```bash
cd apps/dashboard
bun install
bun run dev
```

Open <http://127.0.0.1:8081/>. You can also open <http://127.0.0.1:5173/>; Vite sends `/api` requests to the admin API on port 8081.

To build the dashboard instead of running Vite:

```bash
cd apps/dashboard
bun install
bun run build
```

Restart go-mamacord after the build, then open <http://127.0.0.1:8081/>.

## Enable Discord sign-in

Add these values to `.env.dev`:

```dotenv
MAMACORD_DASHBOARD_CLIENT_ID=your-application-id
MAMACORD_DASHBOARD_CLIENT_SECRET=your-client-secret
```

In the Discord Developer Portal, add these OAuth2 redirect URLs exactly as shown:

```text
http://127.0.0.1:8081/api/auth/callback
http://127.0.0.1:8081/api/install/callback
```

Restart go-mamacord after changing `.env.dev`.

If you open the dashboard on port 5173 instead, register the same two paths with `http://127.0.0.1:5173` as the origin.

## Run with Docker Compose

1. Create the production environment file:

   ```bash
   cp .env.prod.example .env.prod
   ```

2. Set `DISCORD_TOKEN` in `.env.prod`.

3. Start PostgreSQL and go-mamacord:

   ```bash
   docker compose up --build
   ```

The default Compose service runs the `control`, `gateway`, and `scheduler` roles. See the [reference guide](docs/reference.md#choose-runtime-roles) for split-role deployments.

## Configure a production dashboard

Production mode requires complete dashboard settings when `MAMACORD_ADMIN_ADDR` is set:

```dotenv
MAMACORD_PROD_MODE=1
MAMACORD_ALLOW_UNSIGNED_PLUGINS=0
MAMACORD_ADMIN_ADDR=0.0.0.0:8081
MAMACORD_DASHBOARD_CLIENT_ID=your-application-id
MAMACORD_DASHBOARD_CLIENT_SECRET=your-client-secret
MAMACORD_DASHBOARD_SESSION_SECRET=replace-with-a-random-secret-at-least-32-characters
MAMACORD_PUBLIC_DASHBOARD_ORIGIN=https://example.com
MAMACORD_PUBLIC_API_ORIGIN=https://api.example.com
MAMACORD_DASHBOARD_ALLOWED_ORIGINS=https://example.com
```

For a separately hosted dashboard, set `api_origin` in [`apps/dashboard/public/config.json`](apps/dashboard/public/config.json) to the public API origin before building it:

```json
{
  "api_origin": "https://api.example.com"
}
```

Register these production OAuth2 redirects in Discord:

```text
https://api.example.com/api/auth/callback
https://api.example.com/api/install/callback
```

See the [reference guide](docs/reference.md) for Docker, plugin signing, commands, and runtime settings. See the [SBC hosting guide](docs/sbc-hosting.md) for Raspberry Pi, Orange Pi, and ODROID installation and updates.

## Check the configuration

Run:

```bash
go run ./cmd/mamacord doctor
```

`doctor` reports the loaded environment file, database target, runtime roles, admin API status, and dashboard OAuth settings. It hides the PostgreSQL password.

## Troubleshooting

### PostgreSQL connection fails

Start the included database with `docker compose up -d postgres`. If you use another PostgreSQL server, update `MAMACORD_POSTGRES_DSN` in the active environment file.

### The bot exits with `4014 Disallowed intent(s)`

Enable **Server Members Intent** in the Discord Developer Portal under **Bot → Privileged Gateway Intents**, then restart the bot.

### The dashboard returns `502 Bad Gateway`

Start the dashboard with `cd apps/dashboard && bun run dev`, or build it with `bun run build` and restart go-mamacord.

### The dashboard says authentication is not configured

Set `MAMACORD_DASHBOARD_CLIENT_ID` and `MAMACORD_DASHBOARD_CLIENT_SECRET`, then restart go-mamacord. In production, also set the session secret and public origin values shown above.

### Discord rejects an OAuth redirect

Check the scheme, host, port, and path. The registered URL must exactly match the dashboard URL you opened and the callback path shown above.

## License

go-mamacord is available under the [MIT License](LICENSE).
