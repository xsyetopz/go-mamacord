# Reference

Use this guide to choose a runtime layout, manage commands, and build Starlark plugins. For the first local run, start with the [README](../README.md).

## Dashboard Coverage

Discord sign-in gives each user a server list limited to servers they can manage. A server dashboard shows the bot install state and setup checks. Server managers can:

- configure the `manager`, `moderation`, `fun`, `info`, and `wellness` plugins for that server
- set slowmode and nicknames
- create, edit, delete, add, or remove roles
- purge messages
- create, edit, or delete custom emojis and stickers
- add and remove moderation warnings

Bot owners also get pages for:

- runtime and build status
- enabling, disabling, resetting, and reloading modules
- plugin load and signature status, plugin reload, plugin signing, and plugin scaffolding
- admin API, OAuth, path, and runtime setup checks
- applied and pending database migration status

The dashboard does not apply migrations from its migrations page. The page reports status only.

## Run with Docker Compose

1. Copy the production environment template:

   ```bash
   cp .env.prod.example .env.prod
   ```

2. Set `DISCORD_TOKEN` when either the `gateway` or `scheduler` role will run.
3. Add the dashboard variables described in [Deploy the dashboard](#deploy-the-dashboard) if the `control` role will expose the admin API.
4. Start the default single service:

   ```bash
   docker compose up --build
   ```

The default `mamacord` service runs `control`, `gateway`, and `scheduler`. To change that set for the single service, pass `MAMACORD_RUNTIME_ROLES`:

```bash
MAMACORD_RUNTIME_ROLES=gateway,scheduler docker compose up --build
```

To run one container per role, name the three profiled services explicitly:

```bash
docker compose --profile split up --build mamacord-control mamacord-gateway mamacord-scheduler
```

Do not use `docker compose --profile split up` without service names for this layout. Compose would also start the unprofiled, all-role `mamacord` service.

The Compose file starts PostgreSQL and gives every MamaCord service these settings:

```dotenv
MAMACORD_STORAGE_BACKEND=postgres
MAMACORD_POSTGRES_DSN=postgres://mamacord:secret@postgres:5432/mamacord?sslmode=disable
```

It reads `.env.prod` and mounts:

- `./data` at `/data`; user-installed plugins are under `/data/plugins`
- `./config` at `/app/config` as read-only

First-party plugins remain in the image at `/app/plugins`. The image also sets the bundle store to `/data/bundles/store` and the bundle cache to `/data/bundles/cache`.

For a split deployment, add this to `.env.prod` so the role containers share bundle artifacts:

```dotenv
MAMACORD_BUNDLE_BACKEND=cached
MAMACORD_BUNDLE_STORE_DIR=/data/bundles/store
MAMACORD_BUNDLE_CACHE_DIR=/data/bundles/cache
```

## Choose Runtime Roles

`MAMACORD_RUNTIME_ROLES` accepts a comma-separated set of these roles:

| Role | Work performed |
| --- | --- |
| `control` | Runs the admin API when `MAMACORD_ADMIN_ADDR` is also set. |
| `gateway` | Connects to Discord and receives interactions and gateway events. |
| `scheduler` | Runs scheduled background work. |

The default is `control,gateway,scheduler`. `DISCORD_TOKEN` is required when `gateway` or `scheduler` is enabled. It is not required for a control-only process.

Examples:

```dotenv
# Admin API only
MAMACORD_RUNTIME_ROLES=control

# Discord ingress only
MAMACORD_RUNTIME_ROLES=gateway

# Background scheduler only
MAMACORD_RUNTIME_ROLES=scheduler

# Bot and scheduler without the admin API
MAMACORD_RUNTIME_ROLES=gateway,scheduler
```

`mamacord doctor` and the owner dashboard report the active roles.

## Deploy the Dashboard

### Use the local development server

When `apps/dashboard/dist/index.html` is absent, the admin API proxies dashboard requests to Vite at `http://127.0.0.1:5173`. Start Vite with:

```bash
cd apps/dashboard
bun install
bun run dev
```

The Vite server also proxies `/api` to `http://127.0.0.1:8081`. The [README](../README.md#open-the-local-dashboard) has the full local OAuth setup.

### Serve a built dashboard from the admin API

Build the frontend:

```bash
cd apps/dashboard
bun install
bun run build
```

Run MamaCord from a working directory that contains `apps/dashboard/dist/index.html`. The admin API serves that directory on its own origin instead of proxying to Vite.

The current `Dockerfile` does not copy `apps/dashboard/dist` into the runtime image. A standard Compose build therefore needs a separate static dashboard host or a custom image or mount that supplies the built directory. `compose.yml` also does not publish the admin API port. Add a port mapping or connect a reverse proxy to the Compose network when the API must be reachable outside that network.

### Host the dashboard separately

You can host `apps/dashboard/dist` at the root of a static-site origin and run the admin API on a separate origin. The dashboard uses root-relative asset and configuration URLs, so a repository subpath such as `https://user.github.io/repository/` is not a supported target without additional build configuration. The repository's Pages workflow publishes `apps/site`, not the dashboard.

Before building the dashboard, set `api_origin` in `apps/dashboard/public/config.json`:

```json
{
  "api_origin": "https://api.example.com"
}
```

For a production admin API, set:

```dotenv
MAMACORD_PROD_MODE=1
MAMACORD_ALLOW_UNSIGNED_PLUGINS=0
MAMACORD_ADMIN_ADDR=0.0.0.0:8081
MAMACORD_DASHBOARD_CLIENT_ID=your-application-id
MAMACORD_DASHBOARD_CLIENT_SECRET=your-client-secret
MAMACORD_DASHBOARD_SESSION_SECRET=at-least-32-characters
MAMACORD_PUBLIC_DASHBOARD_ORIGIN=https://example.com
MAMACORD_PUBLIC_API_ORIGIN=https://api.example.com
MAMACORD_DASHBOARD_ALLOWED_ORIGINS=https://example.com
```

`MAMACORD_DASHBOARD_SESSION_SECRET` must contain at least 32 characters. The allowed-origins list must include the dashboard origin. Register these Discord OAuth2 redirect URLs for the API origin:

```text
https://api.example.com/api/auth/callback
https://api.example.com/api/install/callback
```

For SBC and cross-build deployments, see the [SBC hosting guide](sbc-hosting.md).

## Build a Release Binary

Run:

```bash
./scripts/build-release.sh
```

The default output is `dist/mamacord`. Pass a path as the first argument to choose another output:

```bash
./scripts/build-release.sh dist/mamacord-linux-arm64
```

The script injects build information. Override any field with an environment variable:

- `VERSION`
- `REPOSITORY`
- `DESCRIPTION`
- `DEVELOPER_URL`
- `SUPPORT_SERVER_URL`
- `MASCOT_IMAGE_URL`

The Docker build uses the corresponding arguments:

| Release script | Docker build argument |
| --- | --- |
| `VERSION` | `BUILD_VERSION` |
| `REPOSITORY` | `BUILD_REPOSITORY` |
| `DESCRIPTION` | `BUILD_DESCRIPTION` |
| `DEVELOPER_URL` | `BUILD_DEVELOPER_URL` |
| `SUPPORT_SERVER_URL` | `BUILD_SUPPORT_SERVER_URL` |
| `MASCOT_IMAGE_URL` | `BUILD_MASCOT_IMAGE_URL` |

## Use Discord Commands

### Built-in commands

The `core` module provides:

- `/ping`
- `/help`

The owner-only `admin` module provides:

- `/block` and `/unblock`
- `/plugins`
- `/modules`

`core` and `admin` are required modules and cannot be disabled.

### First-party plugin commands

First-party plugins are stored under `plugins/`. Their top-level commands are:

- `info`: `/about`, `/lookup user`, `/lookup guild`, `/lookup role`, `/lookup channel`
- `fun`: `/flip`, `/roll`, `/8ball`, `/hug`, `/pat`, `/poke`, `/shrug`
- `wellness`: `/timezone`, `/checkin`, `/remind`
- `moderation`: `/warn`, `/unwarn`
- `manager`: `/slowmode`, `/nick`, `/purge`, `/roles`, `/emojis`, `/stickers`

`config/modules.json` enables `info` by default and disables the other four first-party plugins. Use `/modules`, the owner dashboard, or stored module state to change those choices.

## Manage Modules and Reload Plugins

MamaCord treats both built-ins and plugins as modules.

- `config/modules.json` supplies the default module state. Override its path with `MAMACORD_MODULES_FILE`.
- Module changes made at runtime are stored in PostgreSQL and take precedence over the file defaults.
- `/modules enable`, `/modules disable`, and `/modules reset` change one toggleable module.
- `/modules reload` and `/plugins reload` are owner-only. Both load and verify all plugins, rebuild the module catalog, update Discord command registration when the gateway is running, and restart the scheduler when it is present and ready.

A plugin reload compiles, initializes, and sets up a new immutable runtime generation before swapping it into use. The old generation then drains for up to three seconds before cleanup.

In a split-role deployment, a control-only process has no Discord plugin runtime. Dashboard module and plugin reload actions therefore fail on that node, and bundle backends do not broadcast reloads to gateway or scheduler processes. Run the owner-only `/plugins reload` command through the gateway process to activate shared bundle changes there; restart other role processes when they also need the new generation.

## Configure Command Registration

Commands register globally by default. `DISCORD_DEV_GUILD_ID` overrides the registration mode and registers commands only in that guild.

For other layouts, use:

- `MAMACORD_COMMAND_REGISTRATION_MODE=global|guilds|hybrid`
- `MAMACORD_COMMAND_GUILD_IDS=123,456` for the `guilds` and `hybrid` modes
- `MAMACORD_COMMAND_REGISTER_ALL_GUILDS=1` to also register in every guild in the gateway cache when no development guild is set

`hybrid` registers the same command set globally and in the listed guilds.

## Configure Interaction Cooldowns

The slash cooldown is tracked per user and per command path. Default cooldowns are 5,000 ms for slash commands, 750 ms for components, and 1,500 ms for modals. Change them with:

- `MAMACORD_SLASH_COOLDOWN_MS`
- `MAMACORD_COMPONENT_COOLDOWN_MS`
- `MAMACORD_MODAL_COOLDOWN_MS`

`MAMACORD_SLASH_COOLDOWN_BYPASS` is a comma-separated list of command paths that skip the slash cooldown. Its default is `ping,help,plugins,modules,block,unblock`.

`MAMACORD_SLASH_COOLDOWN_OVERRIDES_MS` accepts comma-separated `path=milliseconds` entries:

```dotenv
MAMACORD_SLASH_COOLDOWN_OVERRIDES_MS=lookup:user=2500,roles:add=1000
```

## Create a Starlark Plugin

User plugins go under `data/plugins/<plugin-id>/` by default. `MAMACORD_USER_PLUGINS_DIR` changes that root; `PLUGINS_DIR` is its fallback when that variable is unset. Bundled first-party plugins use the separate `plugins/` root, controlled by `MAMACORD_BUNDLED_PLUGINS_DIR`.

A plugin root records one active filesystem bundle revision:

```text
data/plugins/example/
├── .mamacord-bundle.json
└── bundles/
    └── example-v0.1.0/
        ├── plugin.json
        ├── plugin.star
        ├── commands/
        │   └── hello.star
        ├── locales/
        │   └── en-US/
        │       └── messages.json
        └── signature.json
```

`.mamacord-bundle.json` names the active filesystem bundle directory. A marketplace install record in PostgreSQL takes precedence when it selects another revision; the state file is the fallback selector. The bundle directory itself is not made read-only. The runtime hashes the bundle before and after loading it and rechecks its signature when one is present. This detects changes made during loading.

`plugin.json` is the strict manifest, and its `entrypoint` must be `plugin.star`. Each locale listed in `locales.supported` must have one `locales/<locale>/messages.json` file. A signed bundle contains `bundles/<revision>/signature.json` relative to the plugin root.

Use `examples/plugins/example/` as a fuller working example. The owner dashboard can also scaffold a starter under the configured user plugin root.

### Declare the plugin entrypoint

Repository plugins keep `plugin.star` focused on setup and load command declarations from modules named for their concern:

```starlark
# plugin.star
load("@mamacord//api.star", "cog", "plugin")
load("//commands:hello.star", "HELLO_COMMAND")


def setup(bot):
    bot.add_cog(cog(
        name="Example",
        commands=[HELLO_COMMAND],
    ))


PLUGIN = plugin(setup=setup)
```

```starlark
# commands/hello.star
load("@mamacord//api.star", "reply", "slash_command")


def hello(ctx):
    return [reply(
        content="Hello",
        ephemeral=True,
    )]


HELLO_COMMAND = slash_command(
    name="hello",
    description="Say hello",
    handler=hello,
)
```

A bundle can load only the host module `@mamacord//api.star` and canonical bundle-relative labels such as `//commands:hello.star`. Bundle-relative labels must name visible `.star` files and cannot escape the bundle.

### Request and grant capabilities

A plugin receives a capability only when both of these files allow it:

1. The plugin requests the capability in `plugin.json`.
2. The host grants it through `config/permissions.json`, or the file selected by `MAMACORD_PERMISSIONS_FILE`.

The effective permission set is the intersection of those two sources. A request alone does not grant access.

Plugins do not receive ambient filesystem, process, environment, database, secret, Go object, or Discord client access. They use immutable context values for reads and return typed effects for work that the Go host validates and executes. Network reads also require `network.http`, a manifest host allowlist, and a matching host permission grant.

To read a bundled asset:

1. List its exact relative path in `plugin.json` under `assets`.
2. Request and grant `resources.read`.
3. Call `ctx.resource(path)`.

Bundles cannot contain symlinks or files that are not a source file, a declared locale file, a declared asset, `plugin.json`, or `signature.json`.

## Work Within Plugin Limits

The runtime applies these fixed limits to each loaded bundle:

| Item | Limit |
| --- | ---: |
| Files in a bundle | 512 |
| `plugin.json` | 64 KiB |
| `signature.json` | 16 KiB |
| One `.star` file | 256 KiB |
| All `.star` source | 1 MiB |
| Starlark modules | 64 |
| Module load depth | 32 |
| One locale messages file | 512 KiB |
| One declared asset | 1 MiB |
| All declared assets | 8 MiB |
| Resource path passed to `ctx.resource` | 240 bytes |

Execution limits use both a Starlark step budget and a wall-clock timeout:

| Phase | Steps | Timeout |
| --- | ---: | ---: |
| Module initialization | 250,000 | 500 ms |
| Plugin setup | 250,000 | 500 ms |
| Command, component, modal, event, or job callback | 1,000,000 | 2 s |
| Check or autocomplete callback | 100,000 | 250 ms |

Each Starlark execution emits at most 20 print messages of 1,024 bytes each. Each callback can make at most 100 host calls and return at most 25 operations.

Typed values have a maximum nesting depth of 16 and an aggregate limit of 500 items. One stored state value can encode to at most 16 KiB. Event data and aggregate state input are each limited to 64 KiB per invocation.

`ctx.http_get_json` uses a two-second HTTP client timeout. Its `max_bytes` argument defaults to 64 KiB and cannot exceed 64 KiB. Response headers are also limited to 64 KiB. The client does not follow redirects.

These controls do not impose a hard heap quota for each plugin. Run untrusted plugins in a separate process or container when you need memory isolation.

## Choose a Bundle Backend

`MAMACORD_BUNDLE_BACKEND` accepts three values. All three modes discover plugins from the configured bundled and user plugin roots. Cached and object-store modes move bundle artifacts; they do not replace the local or synchronized plugin roots and their `.mamacord-bundle.json` state.

### Keep bundles in each plugin root

`local` is the default. It loads the selected bundle directly from the plugin root. A stored install revision is selected first when present; otherwise the runtime uses the directory in `.mamacord-bundle.json`:

```dotenv
MAMACORD_BUNDLE_BACKEND=local
```

### Share artifacts and keep a worker cache

`cached` checks the plugin root first, then the shared bundle store. Runtime workers copy the active bundle into the cache before loading it:

```dotenv
MAMACORD_BUNDLE_BACKEND=cached
MAMACORD_BUNDLE_STORE_DIR=/path/to/shared/bundle-store
MAMACORD_BUNDLE_CACHE_DIR=/path/to/worker-local/bundle-cache
```

Use a shared store path and a worker-local cache path in a split-role deployment.

### Use the directory-backed object-store adapter

`objectstore` writes bundle contents through the repository's object-store interface. Its implemented adapter is directory-backed and stores the objects under `MAMACORD_BUNDLE_STORE_DIR`:

```dotenv
MAMACORD_BUNDLE_BACKEND=objectstore
MAMACORD_BUNDLE_STORE_DIR=/path/to/object-store-root
MAMACORD_BUNDLE_CACHE_DIR=/path/to/worker-local/bundle-cache
```

Artifact materialization uses `<cache>/artifacts/...`; the runtime copy uses `<cache>/active/...`. CLI and dashboard signing update `signature.json` in the canonical store and refresh the materialized copies.

## Sign Plugins for Production

The daemon refuses to start when `MAMACORD_PROD_MODE=1` and `MAMACORD_ALLOW_UNSIGNED_PLUGINS=1`. In a running production daemon, a missing plugin signature is rejected. Any signature that is present is parsed and verified in every mode.

For production, keep:

```dotenv
MAMACORD_ALLOW_UNSIGNED_PLUGINS=0
```

The bundled first-party plugins are already signed. Their trusted public key is in `config/trusted_keys.json`, which is also the default path used by `MAMACORD_TRUSTED_KEYS_FILE`.

### Generate a signer

Run:

```bash
go run ./cmd/mamacord gen-signing-key --key-id your-key-id
```

By default, this command:

- writes the private key to `data/keys/your-key-id.key`
- adds or updates the public key in `config/trusted_keys.json`

Use `--private-key-file` or `--trusted-keys-file` to choose other paths. Keep the private key out of source control.

### Sign the active bundle

For a user plugin named `my-plugin` in the default root, run:

```bash
go run ./cmd/mamacord sign-plugin \
  --dir ./data/plugins/my-plugin \
  --key-id your-key-id \
  --private-key-file ./data/keys/your-key-id.key
```

The command reads `.mamacord-bundle.json`, signs the active bundle, and writes `bundles/<revision>/signature.json`. It uses the configured bundle backend, store, and cache settings. Export the same `MAMACORD_BUNDLE_*` values used by the running deployment when the backend is not `local`.

Trusted public keys can come from the configured trusted-keys file or the PostgreSQL `trusted_signers` store.

### Let the dashboard sign

Set both values:

```dotenv
MAMACORD_DASHBOARD_SIGNING_KEY_ID=your-key-id
MAMACORD_DASHBOARD_SIGNING_KEY_FILE=./data/keys/your-key-id.key
```

The key file must exist on the machine that runs the control role. For a host-run process from the repository root, `./data/keys/your-key-id.key` is correct. Under the supplied Compose mount, use `/data/keys/your-key-id.key` because the container working directory is `/app` and host `./data` is mounted at `/data`.

The dashboard can then sign an existing user plugin or sign a new scaffold during creation.

For an SBC installation, see [Sign Your Own Plugins](sbc-hosting.md#sign-your-own-plugins).

## Validate Configuration JSON

Use these schemas for editor validation:

| File | Schema |
| --- | --- |
| `<plugin-root>/bundles/<revision>/plugin.json` | `schemas/plugin.schema.json` |
| `<plugin-root>/bundles/<revision>/locales/<locale>/messages.json` | `schemas/messages.schema.json` |
| `config/permissions.json` | `schemas/permissions.schema.json` |
| `config/modules.json` | `schemas/modules.schema.json` |
| `config/trusted_keys.json` | `schemas/trusted_keys.schema.json` |
| `<plugin-root>/bundles/<revision>/signature.json` | `schemas/signature.schema.json` |

The Go loaders decide whether a file is accepted at runtime. They also enforce rules that JSON Schema cannot express fully. For example, some manifest arrays must use canonical ordering, and `locales.default` must appear in `locales.supported`.
