# Reference

This file intentionally contains the longer “power user” documentation that
would otherwise make the main `README.md` hard to scan.

## Dashboard Coverage (Today)

- sign in with Discord
- server picker for guilds the user can manage
- per-server install/setup status
- server settings (plugin config per guild)
- server manager actions: slowmode, nick, roles, purge, emojis, stickers
- moderation actions: warn/unwarn
- owner overview / runtime state
- owner module enable / disable / reset / reload
- owner plugin list / reload / signing state
- owner plugin scaffolding
- owner setup diagnostics for API + OAuth wiring
- owner migration status / backup

## Release Builds

Use `./scripts/build-release.sh` to build a binary with injected `buildinfo` metadata.

Supported env overrides:

- `VERSION`
- `REPOSITORY`
- `DESCRIPTION`
- `DEVELOPER_URL`
- `SUPPORT_SERVER_URL`
- `MASCOT_IMAGE_URL`

The Docker build accepts the same values as `BUILD_*` args.

For SBC and cross-build deployment guidance, see:

- `docs/sbc-hosting.md`

## Deployment Shapes

### Local / Dev Default

- admin API serves or proxies the dashboard on the same origin
- this is the primary development path
- simplest for local cookies, redirects, and debugging

### Canonical Public Production Topology

- static dashboard on GitHub Pages or similar
- separate admin API origin
- preferred domain shape: `example.com` + `api.example.com`
- this is the repo's main public deployment recommendation

Raw `*.github.io` hosting is supported, but discouraged as the main/default
public path. Prefer a custom domain if you want GitHub Pages to be the primary
public dashboard host.

### Self-Hosted / Single-Box

- admin API serves built `apps/dashboard/dist`
- best fit for LAN, homelab, SBCs, and single-machine setups
- simpler operationally than split hosting, but not the canonical public shape

## Runtime Roles

MamaCord now supports split runtime-role boot from one daemon config surface:

- `control`
- `gateway`
- `scheduler`

Configure them with:

- `MAMACORD_RUNTIME_ROLES=control,gateway,scheduler` (default)

Examples:

- control-only API node: `MAMACORD_RUNTIME_ROLES=control`
- gateway-only Discord ingress node: `MAMACORD_RUNTIME_ROLES=gateway`
- scheduler-only background worker: `MAMACORD_RUNTIME_ROLES=scheduler`
- single-node bot + scheduler without dashboard: `MAMACORD_RUNTIME_ROLES=gateway,scheduler`

Notes:

- `DISCORD_TOKEN` is required only when `gateway` or `scheduler` is enabled
- `MAMACORD_ADMIN_ADDR` only starts the admin API when the `control` role is enabled
- dashboard status and `mamacord doctor` report the active runtime-role set

## Docker

1. Copy `.env.prod.example` to `.env.prod`.
2. If `gateway` or `scheduler` is in `MAMACORD_RUNTIME_ROLES`, fill in `DISCORD_TOKEN`.
3. If you want the admin API in Docker, also fill in the required
   `MAMACORD_DASHBOARD_*` and public origin vars.
4. Start: `docker compose up --build`

For split-role boot from Compose:

- single service with explicit roles:
  - `MAMACORD_RUNTIME_ROLES=control,gateway,scheduler docker compose up --build`
- separate role containers:
  - `docker compose --profile split up --build`

`compose.yml` now starts a `postgres` service and points every runtime role at:

- `MAMACORD_STORAGE_BACKEND=postgres`
- `MAMACORD_POSTGRES_DSN=postgres://mamacord:secret@postgres:5432/mamacord?sslmode=disable`

For shared/split deployments, also set:

- `MAMACORD_BUNDLE_BACKEND=cached`
- `MAMACORD_BUNDLE_STORE_DIR=/data/bundles/store`
- `MAMACORD_BUNDLE_CACHE_DIR=/data/bundles/cache`

`compose.yml` now reads `.env.prod` and bind-mounts:

- `./data` → `/data`, including mutable/user-installed plugins at `./data/plugins`
- `./config` → `/app/config` (read-only)

Bundled first-party plugins stay in the image at `/app/plugins`.

## Built-in Commands

- `/ping`
- `/help`
- `/block` and `/unblock` (owner-only)
- `/plugins`
- `/modules`

Optional first-party plugins live in `plugins/` too:

- `info`: `/about`, `/lookup user|guild|role|channel`
- `fun`: `/flip`, `/roll`, `/8ball`, `/hug`, `/pat`, `/poke`, `/shrug`
- `wellness`: `/timezone`, `/checkin`, `/remind`
- `moderation`: `/warn`, `/unwarn`
- `manager`: `/slowmode`, `/nick`, `/purge`, `/roles`, `/emojis`, `/stickers`

## Modules

mamacord treats built-ins and plugins as modules.

Default module seeds: `config/modules.json`
Runtime overrides: stored in the configured storage backend.

## Starlark Plugins

Runtime plugin roots live under `plugins/<plugin>/`.

Each root contains:

- `.mamacord-bundle.json`, which selects one immutable bundle revision
- `bundles/<revision>/plugin.json`, a strict `Mamacord Starlark` manifest
- `bundles/<revision>/plugin.star`, the fixed entrypoint
- `bundles/<revision>/locales/<locale>/messages.json` for every declared locale
- exactly the assets declared by the manifest, if any
- `signature.json` for signed bundles

The entrypoint is declarative and loads complete declarations from concern-specific modules:

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

`plugin.star` owns setup only. Commands, components, listeners, tasks, and their helpers live in modules named for their concern. A plugin with declared assets and an effective `resources.read` grant can read an exact manifest-declared asset as bounded immutable bytes with `ctx.resource(path)`.

Only `@mamacord//api.star` and canonical bundle-relative `//path:module.star` labels may load. Bundles cannot contain symlinks or undeclared files. Plugins have no ambient filesystem, network, process, environment, database, secret, Go-object, or Discord-client access.

Callbacks receive immutable typed context values. Reads use bounded capability-gated context methods. Callbacks return ordered typed effects, which Go validates and executes. Capability authority is the intersection of the strict manifest request and `config/permissions.json`.

Runtime compilation, initialization, setup, callbacks, host calls, values, source bytes, module count, load depth, output operations, and HTTP response bytes are bounded. Reload verifies and initializes a new immutable generation before the atomic swap, then drains and releases the old generation. These limits do not provide a hard per-plugin heap quota; use process or container isolation for hostile-code memory containment.

### Bundle Backends

Default runtime mode is local bundle roots:

- `MAMACORD_BUNDLE_BACKEND=local`
- plugin bundle artifacts live under each plugin root
- runtime loads the active bundle path directly from that root

Shared/split-role installs can switch to cached bundle materialization:

- `MAMACORD_BUNDLE_BACKEND=cached`
- `MAMACORD_BUNDLE_STORE_DIR=/path/to/shared/bundle-store`
- `MAMACORD_BUNDLE_CACHE_DIR=/path/to/worker-local/bundle-cache`

In cached mode:

- the immutable bundle artifact resolves from the shared bundle store (or an existing root-backed bundled/manual source)
- gateway/runtime workers materialize the active bundle into `MAMACORD_BUNDLE_CACHE_DIR`
- admin/plugin signing resolves the bundle artifact dir, not the worker cache dir

Object-store-backed installs can separate canonical bundle storage from local artifact/runtime caches:

- `MAMACORD_BUNDLE_BACKEND=objectstore`
- `MAMACORD_BUNDLE_STORE_DIR=/path/to/object-store-root`
- `MAMACORD_BUNDLE_CACHE_DIR=/path/to/worker-local/bundle-cache`

In objectstore mode:

- canonical bundle contents live behind the bundle object-store adapter
- `ResolveBundleDir` materializes an artifact cache under `bundle-cache/artifacts/...`
- runtime uses `bundle-cache/active/...`
- dashboard / CLI signing writes `signature.json` back through the repository so the canonical artifact and both local caches stay in sync

### JSON Schemas

- `plugins/<plugin>/bundles/<revision>/plugin.json` → `schemas/plugin.schema.json`
- `config/permissions.json` → `schemas/permissions.schema.json`
- `config/modules.json` → `schemas/modules.schema.json`
- `config/trusted_keys.json` → `schemas/trusted_keys.schema.json`
- `plugins/<plugin>/bundles/<revision>/signature.json` → `schemas/signature.schema.json`

These schemas define the canonical JSON structure and the cross-field rules that JSON Schema can express. The Go loaders remain the acceptance authority for data-dependent rules, including canonical ordering and requiring `locales.default` to occur in `locales.supported`.

### Hot Reload

- `/plugins reload` reloads plugins from disk and re-registers commands (owner-only).
- `/modules reload` rebuilds the module catalog and command registration.

### Signing (prod)

When `MAMACORD_PROD_MODE=1`, plugins must be signed.

Fast rules:

- bundled plugins are already signed
- their matching trusted public keys live in `./config/trusted_keys.json`
- custom plugins need your own signer key, then `sign-plugin`

Stock bundled plugins:

- keep `MAMACORD_ALLOW_UNSIGNED_PLUGINS=0`
- make sure `config/trusted_keys.json` is present on the installed machine
- default trusted key path is `./config/trusted_keys.json` unless you override `MAMACORD_TRUSTED_KEYS_FILE`

Generate your own signer:

```bash
go run ./cmd/mamacord gen-signing-key --key-id your-key-id
```

That creates:

- a private key file, by default `./data/keys/your-key-id.key`
- a trusted public key entry in `./config/trusted_keys.json`

Sign a plugin root:

```bash
go run ./cmd/mamacord sign-plugin --dir ./plugins/<id> --key-id your-key-id --private-key-file ./data/keys/your-key-id.key
```

That resolves the active bundle and writes:

- `plugins/<id>/bundles/<revision>/signature.json`

If you want the dashboard to sign scaffolded plugins too, set:

- `MAMACORD_DASHBOARD_SIGNING_KEY_ID=your-key-id`
- `MAMACORD_DASHBOARD_SIGNING_KEY_FILE=./data/keys/your-key-id.key`

Additional trusted keys can also live in the configured storage backend (`trusted_signers`), but file-based trusted keys are the simplest first-boot path.

For SBC/self-hosted production setup, see:

- `docs/sbc-hosting.md#production-plugin-signing`

## Compatibility Options

### Cooldowns

- Global: `MAMACORD_SLASH_COOLDOWN_MS`
- Overrides: `MAMACORD_SLASH_COOLDOWN_OVERRIDES_MS` (comma-separated `name=ms`)

### Command Registration

By default, mamacord registers slash commands globally (unless `DISCORD_DEV_GUILD_ID` is set).

- `MAMACORD_COMMAND_REGISTRATION_MODE=global|guilds|hybrid`
- `MAMACORD_COMMAND_GUILD_IDS=...`
- `MAMACORD_COMMAND_REGISTER_ALL_GUILDS=1`
