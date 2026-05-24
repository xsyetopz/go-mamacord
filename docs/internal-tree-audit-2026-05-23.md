# internal/ tree audit — 2026-05-23

This file is the exported baseline `internal/` snapshot from 2026-05-23. It is not a live tree listing; current implementation progress and later structural changes are tracked in `docs/superpowers/plans/2026-05-23-architecture-alignment.md`.

## Exported tree

```text
internal/
├── adminapi
│   ├── api_errors.go
│   ├── guild_handlers.go
│   ├── guilds.go
│   ├── oauth.go
│   ├── oauth_test.go
│   ├── server.go
│   ├── server_test.go
│   ├── service.go
│   ├── service_test.go
│   └── snowflake.go
├── app
│   ├── app.go
│   └── app_test.go
├── buildinfo
│   ├── buildinfo.go
│   └── buildinfo_test.go
├── commands
│   ├── admin
│   │   └── commands.go
│   ├── api
│   │   └── contracts.go
│   ├── core
│   │   └── commands.go
│   └── catalog.go
├── config
│   ├── config.go
│   ├── config_test.go
│   └── modules.go
├── dotenv
│   ├── dotenv.go
│   └── dotenv_test.go
├── guildconfig
│   └── config.go
├── i18n
│   ├── command_metadata_test.go
│   ├── discord_locales.go
│   ├── i18n.go
│   ├── locale_parity_test.go
│   ├── plugin_locales_test.go
│   └── style_compliance_test.go
├── logging
│   └── logging.go
├── marketplace
│   ├── manager.go
│   ├── manager_test.go
│   └── types.go
├── migration
│   ├── migrate.go
│   └── migrate_test.go
├── ops
│   ├── metrics.go
│   ├── server.go
│   └── server_test.go
├── permissions
│   ├── permissions.go
│   └── permissions_test.go
├── persona
│   └── persona.go
├── runtime
│   ├── discord
│   │   ├── automation
│   │   │   └── reminders.go
│   │   ├── catalog
│   │   │   ├── commands.go
│   │   │   ├── modules.go
│   │   │   └── stats.go
│   │   ├── cdn
│   │   │   ├── cdn.go
│   │   │   └── cdn_test.go
│   │   ├── commands
│   │   │   ├── application.go
│   │   │   ├── registration.go
│   │   │   └── runtime.go
│   │   ├── gateway
│   │   │   └── handlers.go
│   │   ├── interactions
│   │   │   ├── actions.go
│   │   │   ├── kind.go
│   │   │   ├── notice.go
│   │   │   ├── sanitize.go
│   │   │   ├── theme.go
│   │   │   └── update_response.go
│   │   ├── parse
│   │   │   ├── parsing.go
│   │   │   └── parsing_test.go
│   │   ├── plugin
│   │   │   ├── actions.go
│   │   │   ├── automation.go
│   │   │   ├── discord_executor.go
│   │   │   ├── discord_extended.go
│   │   │   ├── discord_messages.go
│   │   │   ├── discord_read.go
│   │   │   ├── errors.go
│   │   │   ├── response.go
│   │   │   ├── response_test.go
│   │   │   ├── slash_interaction.go
│   │   │   └── types.go
│   │   ├── router
│   │   │   ├── autocomplete.go
│   │   │   ├── autocomplete_test.go
│   │   │   ├── cooldown.go
│   │   │   └── input.go
│   │   ├── admin.go
│   │   ├── admin_export.go
│   │   ├── bot.go
│   │   ├── client.go
│   │   ├── command_runtime.go
│   │   ├── component_dispatch.go
│   │   ├── cooldown.go
│   │   ├── cooldowns_policy.go
│   │   ├── gateway_diagnostics.go
│   │   ├── gateway_diagnostics_test.go
│   │   ├── gateway_events.go
│   │   ├── guild_config.go
│   │   ├── lifecycle.go
│   │   ├── metrics_runtime.go
│   │   ├── modal_dispatch.go
│   │   ├── module_state.go
│   │   ├── new_config.go
│   │   ├── owners.go
│   │   ├── owners_test.go
│   │   ├── plugin_runtime.go
│   │   ├── reminders_scheduler.go
│   │   ├── runtime_catalog.go
│   │   ├── services.go
│   │   └── stats.go
│   └── plugins
│       ├── lua
│       │   ├── descriptor.go
│       │   ├── discord_extended.go
│       │   ├── discord_lookup.go
│       │   ├── discord_management.go
│       │   ├── discord_messages.go
│       │   ├── http.go
│       │   ├── moderation_api.go
│       │   ├── routes.go
│       │   ├── runtime_api.go
│       │   ├── sdk.go
│       │   ├── vm.go
│       │   ├── vm_test.go
│       │   └── wellness_api.go
│       ├── custom_id.go
│       ├── host.go
│       ├── host_test.go
│       ├── manifest.go
│       ├── runtime_descriptor.go
│       ├── signing.go
│       ├── signing_cli.go
│       └── signing_test.go
├── scheduling
│   ├── schedule.go
│   └── schedule_test.go
├── postgres
│   ├── postgres.go
│   └── postgres_test.go
├── storage
│   ├── postgres
│   │   ├── admin_sessions.go
│   │   ├── audit.go
│   │   ├── checkins.go
│   │   ├── conv.go
│   │   ├── discord_oauth_tokens.go
│   │   ├── guild_members.go
│   │   ├── guilds.go
│   │   ├── marketplace.go
│   │   ├── module_states.go
│   │   ├── persistence_test.go
│   │   ├── plugin_kv.go
│   │   ├── plugin_oauth_grants.go
│   │   ├── reminders.go
│   │   ├── reminders_test.go
│   │   ├── restrictions.go
│   │   ├── signers.go
│   │   ├── store.go
│   │   ├── user_settings.go
│   │   ├── users.go
│   │   └── warnings.go
│   ├── admin_sessions.go
│   └── store.go
└── timezone
    ├── timezone.go
    └── timezone_test.go
```

## Quick stats

- Go files under `internal/`: 148
- Largest directories by total LOC:
  - `internal/runtime/plugins/lua` — 7787 LOC
  - `internal/adminapi` — 4470 LOC
  - `internal/runtime/discord/plugin` — 3929 LOC
  - `internal/storage/postgres` — 3086 LOC
  - `internal/runtime/plugins` — 2626 LOC
  - `internal/runtime/discord` — 2233 LOC
  - `internal/marketplace` — 1269 LOC
  - `internal/i18n` — 1060 LOC
- Largest non-test files:
  - `internal/runtime/plugins/host.go` — 1523 LOC
  - `internal/adminapi/service.go` — 1517 LOC
  - `internal/runtime/plugins/lua/vm.go` — 1300 LOC
  - `internal/adminapi/server.go` — 1297 LOC
  - `internal/runtime/discord/plugin/response.go` — 1087 LOC
  - `internal/marketplace/manager.go` — 951 LOC
  - `internal/runtime/discord/plugin/discord_executor.go` — 925 LOC
  - `internal/commands/admin/commands.go` — 925 LOC
  - `internal/runtime/plugins/lua/discord_management.go` — 750 LOC
  - `internal/runtime/plugins/lua/discord_extended.go` — 709 LOC

## Directory/package mismatches

- `internal/runtime/discord` uses package `discordruntime` (dir name `discord`)
- `internal/runtime/plugins` uses package `pluginhost` (dir name `plugins`)
- `internal/runtime/plugins/lua` uses package `luaplugin` (dir name `lua`)
- `internal/storage/postgres` uses package `postgresstore` (dir name `postgres`)
- `internal/commands/core` uses package `corecmd` (dir name `core`)
- `internal/commands/api` uses package `commandapi` (dir name `api`)
- `internal/migration` uses package `migrate` (dir name `migration`)


## Structural pressure points from inspected code

- `internal/commands/api/contracts.go` is not a narrow command contract package anymore: it imports Discord event types, `internal/runtime/discord/interactions`, plugin host types, marketplace types, persona helpers, and store interfaces.
- There are two different `commands` trees with different jobs:
  - `internal/commands/...` = builtin command definitions/catalog
  - `internal/runtime/discord/commands/...` = slash registration/dispatch runtime
- The plugin flow is split across three nearby trees that are easy to confuse during navigation:
  - `internal/runtime/plugins` = plugin host/loading/signing
  - `internal/runtime/plugins/lua` = Lua VM + SDK surface
  - `internal/runtime/discord/plugin` = Discord-facing bridge/execution layer
- `internal/adminapi/server.go` and `internal/adminapi/service.go` are both >1200 LOC, so HTTP/session concerns and application/service concerns are already larger than one easy-to-hold-in-context unit.
- `internal/runtime/discord/services.go` and `internal/runtime/discord/commands/runtime.go` both assemble command services / restriction handling, which is a sign the runtime boundary is still blurry.
- `internal/storage/postgres` is large, but it is already table/concern-sliced into focused files; this is not the first place to restructure.

## Go-idiomatic restructuring candidates (priority order)

1. Remove dir/package naming ambiguity first.
2. Untangle builtin command definition code from Discord command runtime code.
3. Make the plugin stack read top-down by layer.
4. Break `adminapi` into route/domain slices.
5. Continue decomposing `internal/runtime/discord` around runtime roles.
