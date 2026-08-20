# Host mamacord on a Raspberry Pi, Orange Pi, or ODROID

The recommended setup uses Docker Compose. It runs one mamacord container and
one PostgreSQL container on the board. This path works the same way on
Raspberry Pi OS, Ubuntu, Debian, and Armbian when Docker supports the OS.

Use a 64-bit OS when the board supports one. A 32-bit ARMv7 OS also works, but
builds and updates take longer.

## Before you start

You need:

- a Raspberry Pi, Orange Pi, or ODROID running Linux
- stable power, working networking, and several gigabytes of free storage
- Docker Engine with the Compose v2 plugin
- Git
- a Discord application and bot token
- the **Server Members Intent** enabled for the bot in the Discord Developer
  Portal

Check the device:

```bash
uname -m
getconf LONG_BIT
docker version
docker compose version
git --version
```

Use this table for the architecture reported by `uname -m`:

| Output | Container platform | Status |
| --- | --- | --- |
| `aarch64` or `arm64` | `linux/arm64/v8` | Recommended |
| `armv7l` or `armv7` | `linux/arm/v7` | Supported, but slower |
| `armv6l` | none in the checked images | Not supported by this Compose setup |

A 64-bit OS is the usual choice for Raspberry Pi 3, 4, 5, and Zero 2 W,
64-bit Orange Pi models, and 64-bit ODROID models. The exact board name does
not matter. The OS architecture does.

The images pinned by this repository all publish `linux/arm64/v8` and
`linux/arm/v7` variants:

- build image:
  `golang:1.26.6-bookworm@sha256:116d58cbd88c1297624acc6e967a060012422bacf9930927e23fb719189c6f36`
- runtime base:
  `debian:bookworm-slim@sha256:abd67ffcfa541b485a3dff59865ab629aa048a6c613e639d36e7456b0b229241`
- database:
  `postgres:18-bookworm@sha256:7d2695c3aa88e792e8b3b233e7e4adb296a20412c6c0ca361e3edaaacfada108`

Install Docker from the instructions for your board's Linux distribution. Add
your login user to the `docker` group if you want to run the commands below
without `sudo`. Log out and back in after changing group membership.

## What Compose stores

Run all Compose commands from the repository checkout. The examples below use
`~/go-mamacord`.

The current `compose.yml` uses these locations:

| Data | Host location | Container location |
| --- | --- | --- |
| PostgreSQL database | Docker volume `go-mamacord_postgres-data` | `/var/lib/postgresql/data` |
| User plugins, bundle data, and cache | `./data` | `/data` |
| Configuration and trusted public keys | `./config` | `/app/config`, read-only |
| Bundled plugins | inside the mamacord image | `/app/plugins` |
| Production settings and secrets | `./.env.prod` | passed as container environment variables |

The image also contains the executable at `/usr/local/bin/mamacord`, migrations
at `/app/migrations`, and locales at `/app/locales`.

Do not use `docker compose down -v` unless you intend to delete the PostgreSQL
volume. A normal `docker compose down` keeps it.

## Recommended install: bot and PostgreSQL on one board

### 1. Clone the repository

Run on the board:

```bash
cd ~
git clone https://github.com/xsyetopz/go-mamacord.git
cd go-mamacord
```

If you already have the checkout, enter it instead:

```bash
cd ~/go-mamacord
```

The mamacord image runs as user and group ID `1000` by default. Most first
users on SBC images also use `1000`. Check with `id -u` and `id -g`. If the
owner of the checkout uses different IDs, set the Dockerfile build arguments
in `compose.override.yml` before the first build:

```yaml
services:
  mamacord:
    build:
      args:
        UID: "replace-with-id-u"
        GID: "replace-with-id-g"
```

This lets the container write to `./data` without making it world-writable.
Keep any later override settings in the same `compose.override.yml` file.

### 2. Create the production settings

```bash
cp .env.prod.example .env.prod
chmod 600 .env.prod
```

Open `.env.prod` in an editor. Set the bot token and use only the gateway and
scheduler roles for a bot-only host:

```dotenv
DISCORD_TOKEN=replace-with-your-discord-bot-token
MAMACORD_RUNTIME_ROLES=gateway,scheduler

MAMACORD_PROD_MODE=1
MAMACORD_ALLOW_UNSIGNED_PLUGINS=0
MAMACORD_STORAGE_BACKEND=postgres
MAMACORD_POSTGRES_DSN=postgres://mamacord:secret@postgres:5432/mamacord?sslmode=disable
```

Keep `postgres` as the database host. `127.0.0.1` would point back to the
mamacord container, not to the PostgreSQL container.

Only `.env.dev` and `.env.prod` are valid mamacord dotenv names. The Compose
setup uses `.env.prod`.

The bundled plugins are already signed. Leave
`MAMACORD_ALLOW_UNSIGNED_PLUGINS=0`, and keep `config/trusted_keys.json` in the
checkout. No signing command is needed for the bundled plugins.

### 3. Check the resolved Compose configuration

Always pass `.env.prod` as the Compose interpolation file. This makes a
`MAMACORD_RUNTIME_ROLES` value in `.env.prod` take effect in `compose.yml`.
The service's `env_file` entry alone does not supply values for Compose
interpolation.

```bash
docker compose --env-file .env.prod config --quiet
```

If this command reports a missing variable or invalid YAML, fix that before
starting the containers.

### 4. Build and start

```bash
docker compose --env-file .env.prod up -d --build
```

Compose waits for PostgreSQL's `pg_isready` health check before it starts
mamacord. The containers use `restart: unless-stopped`, so Docker restarts them
after a reboot unless you stopped them yourself.

A build can take several minutes on a small board. If the build is killed on a
low-memory device, use [Build the image on another computer](#alternative-build-the-image-on-another-computer).

### 5. Verify the install

```bash
docker compose --env-file .env.prod ps
docker compose --env-file .env.prod logs --tail=100 mamacord
docker compose --env-file .env.prod exec mamacord mamacord doctor
```

For the bot-only settings above, `doctor` should include:

```text
discord_token: true
storage_backend: postgres
runtime_roles: gateway,scheduler
prod_mode: true
allow_unsigned_plugins: false
trusted_keys_file_exists: true
```

`env_file_loaded: false` is normal inside Compose. Compose has already placed
the settings in the container environment; mamacord did not read a dotenv file
from its container filesystem.

Check PostgreSQL directly if mamacord cannot connect:

```bash
docker compose --env-file .env.prod exec postgres pg_isready -U mamacord -d mamacord
```

## Routine operation

Run these commands from `~/go-mamacord`.

```bash
# Show container state
docker compose --env-file .env.prod ps

# Follow bot logs
docker compose --env-file .env.prod logs -f mamacord

# Restart only mamacord
docker compose --env-file .env.prod restart mamacord

# Stop the stack without deleting its data volume
docker compose --env-file .env.prod down

# Start it again without rebuilding
docker compose --env-file .env.prod up -d
```

### Optional HTTP health endpoints

The checked `compose.yml` defines a health check for PostgreSQL. It does not
define one for mamacord. `mamacord doctor` checks configuration; it is not a
liveness probe.

Mamacord can expose these endpoints when `MAMACORD_OPS_ADDR` is set:

- `/healthz` returns `200` while the ops HTTP server is running
- `/readyz` returns `200` when the runtime reports ready, otherwise `503`
- `/metrics` returns Prometheus text metrics

To expose them only on the board's loopback interface, create
`compose.override.yml` in the checkout:

```yaml
services:
  mamacord:
    environment:
      MAMACORD_OPS_ADDR: 0.0.0.0:8080
    ports:
      - "127.0.0.1:8080:8080"
```

Recreate the service and test it:

```bash
docker compose --env-file .env.prod up -d
a=$(curl -fsS http://127.0.0.1:8080/healthz) && printf '%s\n' "$a"
curl -fsS http://127.0.0.1:8080/readyz
```

The endpoints have no authentication. Keep the host-side port on
`127.0.0.1` unless a firewall or trusted monitoring network protects it.

## Back up the host

Back up before every update. A complete Compose backup needs both a PostgreSQL
dump and the bind-mounted files. Store the backup outside the repository.

Run while the stack is up:

```bash
cd ~/go-mamacord
umask 077
stamp=$(date +%Y%m%d-%H%M%S)
backup="$HOME/mamacord-backups/$stamp"
mkdir -p "$backup"

git rev-parse HEAD > "$backup/commit"
git branch --show-current > "$backup/branch"
docker compose --env-file .env.prod exec -T postgres \
  pg_dump -U mamacord -d mamacord -Fc > "$backup/postgres.dump"
files=(.env.prod config data)
for path in compose.override.yml compose.prebuilt.yml apps/dashboard/dist; do
  if [[ -e "$path" ]]; then
    files+=("$path")
  fi
done
tar -czf "$backup/files.tar.gz" "${files[@]}"

printf 'Backup: %s\n' "$backup"
```

Check both archives before relying on them:

```bash
docker compose --env-file .env.prod exec -T postgres \
  pg_restore -l < "$backup/postgres.dump" > /dev/null
tar -tzf "$backup/files.tar.gz" > /dev/null
```

Protect this directory. It contains the Discord token and may contain plugin
signing keys.

## Update

First make the backup above. Keep the value of `$backup` or note the printed
path.

Check for local repository edits, then update and rebuild:

```bash
cd ~/go-mamacord
git status --short
git pull --ff-only
docker compose --env-file .env.prod build --pull mamacord
docker compose --env-file .env.prod up -d
docker compose --env-file .env.prod ps
docker compose --env-file .env.prod exec mamacord mamacord doctor
docker compose --env-file .env.prod logs --tail=100 mamacord
```

Stop if `git status --short` shows changes you do not understand. Do not reset
or delete them to force the update.

Mamacord applies pending PostgreSQL migrations during startup. The repository
has forward migrations only. Building an older image does not undo a migration.
Use a database backup for a full rollback.

## Roll back an update

These commands replace the current database and bind-mounted files with one
backup. Confirm that `$backup` points to the matching backup directory before
you run them.

Stop the application, but leave PostgreSQL available:

```bash
cd ~/go-mamacord
backup="$HOME/mamacord-backups/REPLACE-WITH-BACKUP-DIRECTORY"
docker compose --env-file .env.prod stop mamacord
git switch --detach "$(cat "$backup/commit")"
```

Restore the files:

```bash
rm -rf data config apps/dashboard/dist
rm -f .env.prod compose.override.yml compose.prebuilt.yml
tar -xzf "$backup/files.tar.gz"
```

Restore PostgreSQL:

```bash
docker compose --env-file .env.prod up -d postgres
docker compose --env-file .env.prod exec -T postgres \
  dropdb -U mamacord --if-exists mamacord
docker compose --env-file .env.prod exec -T postgres \
  createdb -U mamacord -O mamacord mamacord
docker compose --env-file .env.prod exec -T postgres \
  pg_restore -U mamacord -d mamacord --exit-on-error \
  < "$backup/postgres.dump"
```

Build the recorded revision and verify it:

```bash
docker compose --env-file .env.prod up -d --build mamacord
docker compose --env-file .env.prod ps
docker compose --env-file .env.prod exec mamacord mamacord doctor
docker compose --env-file .env.prod logs --tail=100 mamacord
```

The checkout is now detached at the old commit. After you fix the update
problem, switch back to your normal branch before the next update.

If you use the split-role alternative below, stop all three mamacord role
services before restoring the database.

## Sign your own plugins

Skip this section if you use only the bundled plugins.

Compose stores user plugins under `./data/plugins` on the host. Keep custom
trusted keys under `./data` too, so updates do not modify the tracked
`config/trusted_keys.json` file.

Create a writable trusted-key file with the bundled official key, then point
mamacord to it. Run once:

```bash
cd ~/go-mamacord
cp config/trusted_keys.json data/trusted_keys.json
printf '\nMAMACORD_TRUSTED_KEYS_FILE=/data/trusted_keys.json\n' >> .env.prod
```

Generate a signer. Replace `home` with your own key ID:

```bash
docker compose --env-file .env.prod run --rm mamacord \
  gen-signing-key \
  --key-id home \
  --private-key-file /data/keys/home.key \
  --trusted-keys-file /data/trusted_keys.json
```

Put a custom plugin at `data/plugins/my-plugin`, then sign its active
bundle. Replace `my-plugin` with the plugin directory name:

```bash
docker compose --env-file .env.prod run --rm mamacord \
  sign-plugin \
  --dir /data/plugins/my-plugin \
  --key-id home \
  --private-key-file /data/keys/home.key
```

The signature command writes
`data/plugins/my-plugin/bundles/<revision>/signature.json`. Restart mamacord
after adding or changing a plugin:

```bash
docker compose --env-file .env.prod up -d --force-recreate mamacord
docker compose --env-file .env.prod exec mamacord mamacord doctor
```

Do not copy the private key to another machine unless that machine must sign
plugins. Back up `data/keys`, `data/trusted_keys.json`, and the signed plugin
together.

## Alternative: build the image on another computer

Use this path when `docker compose up --build` runs out of memory on the board.
The build computer needs Docker Buildx and a checkout at the revision you want
to run.

On the build computer, choose the board's platform:

```bash
# 64-bit board OS
platform=linux/arm64

# For a 32-bit ARMv7 OS, use this instead:
# platform=linux/arm/v7

# Use the values reported by id -u and id -g on the board.
target_uid=1000
target_gid=1000

docker buildx build \
  --platform "$platform" \
  --build-arg UID="$target_uid" \
  --build-arg GID="$target_gid" \
  --load \
  -t mamacord:sbc .
docker save mamacord:sbc | gzip > mamacord-sbc.tar.gz
scp mamacord-sbc.tar.gz USER@BOARD:~/
```

On the board, load the image:

```bash
gzip -dc ~/mamacord-sbc.tar.gz | docker load
cd ~/go-mamacord
```

Create `compose.prebuilt.yml`:

```yaml
services:
  mamacord:
    image: mamacord:sbc
    build: !reset null
```

`!reset` requires Docker Compose 2.24 or later. Start with both Compose files:

```bash
docker compose \
  -f compose.yml \
  -f compose.prebuilt.yml \
  --env-file .env.prod \
  up -d
```

Use the same `-f compose.yml -f compose.prebuilt.yml` options for later Compose
commands while this alternative is active. PostgreSQL still uses the pinned
image from `compose.yml` and keeps the same data volume.

For an update, make a backup, update the checkout on both machines to the same
commit, and repeat the build, save, copy, load, and combined `up -d` commands.
Do not run the normal Compose build command on the low-memory board.

The repository's release script can also create standalone Linux binaries. Its
verified cross-build commands are:

```bash
GOOS=linux GOARCH=arm64 ./scripts/build-release.sh dist/mamacord-linux-arm64
GOOS=linux GOARCH=arm GOARM=7 ./scripts/build-release.sh dist/mamacord-linux-armv7
```

A standalone binary still needs PostgreSQL, the repository assets, production
settings, and a service manager. Compose is the simpler complete install.

## Alternative: enable the admin API and built dashboard

The recommended bot-only setup does not publish an admin port and the Docker
image does not contain `apps/dashboard/dist`. Enable this only if you also
provide HTTPS and the Discord OAuth redirect settings described in the main
`README.md`.

Use all runtime roles in `.env.prod` and set every required production value:

```dotenv
MAMACORD_RUNTIME_ROLES=control,gateway,scheduler
MAMACORD_ADMIN_ADDR=0.0.0.0:8081
MAMACORD_DASHBOARD_CLIENT_ID=replace-me
MAMACORD_DASHBOARD_CLIENT_SECRET=replace-me
MAMACORD_DASHBOARD_SESSION_SECRET=replace-with-at-least-32-characters
MAMACORD_PUBLIC_DASHBOARD_ORIGIN=https://bot.example.com
MAMACORD_PUBLIC_API_ORIGIN=https://bot.example.com
MAMACORD_DASHBOARD_ALLOWED_ORIGINS=https://bot.example.com
```

Build the static dashboard on a computer with Bun:

```bash
bun install --frozen-lockfile
bun run --cwd apps/dashboard build
```

Copy `apps/dashboard/dist` to the same path in the board's checkout if you
built it elsewhere. Add this to `compose.override.yml`:

```yaml
services:
  mamacord:
    ports:
      - "127.0.0.1:8081:8081"
    volumes:
      - ./apps/dashboard/dist:/app/apps/dashboard/dist:ro
```

Then recreate mamacord:

```bash
docker compose --env-file .env.prod up -d
docker compose --env-file .env.prod exec mamacord mamacord doctor
```

The port is bound to loopback for a reverse proxy on the board. Do not expose
the plain HTTP port directly to the internet.

## Alternative: split runtime roles

One container with `gateway,scheduler` is enough for an SBC bot. Split roles
use more memory and are intended for advanced deployments.

If you still need them, add this to `.env.prod`:

```dotenv
MAMACORD_BUNDLE_BACKEND=cached
MAMACORD_BUNDLE_STORE_DIR=/data/bundles/store
MAMACORD_BUNDLE_CACHE_DIR=/data/bundles/cache
```

Start only the named split services:

```bash
docker compose --env-file .env.prod stop mamacord
docker compose --env-file .env.prod rm -f mamacord
docker compose --profile split --env-file .env.prod up -d --build \
  postgres mamacord-control mamacord-gateway mamacord-scheduler
```

Do not use `docker compose --profile split up` without the service names. An
unqualified `up` also starts the normal unprofiled `mamacord` service, which
would duplicate the roles.

## Troubleshooting

### The image build is killed

The board probably ran out of memory. Build the image on another computer as
shown above. This is common on 512 MB boards.

### `exec format error`

The image or standalone binary was built for the wrong architecture. Check
`uname -m`, then rebuild for `linux/arm64` or `linux/arm/v7` as shown above.

### PostgreSQL stays unhealthy

```bash
docker compose --env-file .env.prod logs --tail=100 postgres
docker compose --env-file .env.prod exec postgres \
  pg_isready -U mamacord -d mamacord
```

Do not change only the DSN in `.env.prod`. The current Compose file sets the
PostgreSQL user, password, database, and mamacord DSN together.

### `doctor` reports `discord_token: false`

Make sure `DISCORD_TOKEN` is set in `~/go-mamacord/.env.prod`, then recreate the
service:

```bash
docker compose --env-file .env.prod up -d --force-recreate mamacord
```

### `doctor` reports no trusted keys

For bundled plugins, confirm that the tracked file exists:

```bash
ls -l config/trusted_keys.json
docker compose --env-file .env.prod exec mamacord mamacord doctor
```

For custom plugins, confirm that `.env.prod` points to
`/data/trusted_keys.json` and that `data/trusted_keys.json` exists on the host.

### The dashboard returns `502` or does not load

The image does not contain dashboard files by default. Build the dashboard and
mount `apps/dashboard/dist` as shown above. Also confirm that all required
production OAuth, session, origin, and port settings are present.
