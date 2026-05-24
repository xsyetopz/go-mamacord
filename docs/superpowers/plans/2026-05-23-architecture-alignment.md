# Architecture Alignment Implementation Tracker

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move MamaCord from the architecture audit in `docs/architecture-audit-2026-05-23.md` into verified code and deployment changes, starting with concrete breaking refactors and repo-truth fixes.

**Architecture:** Execute the audit in slices that each leave the repo in a more truthful, less coupled state. Prioritize repo-truth and deploy-boundary fixes first, then decompose `adminapi`, then untangle command/module/plugin boundaries, then unify scheduling and scale-oriented adapters.

**Tech Stack:** Go, Bun/Vite/React, Postgres, Docker, GitHub Actions, Lua plugin runtime

---

## Progress rules

- Check a box only after the code/doc/test state proves it.
- Keep this tracker aligned with actual repo state.
- Prefer finishing the current slice before opening more code fronts.

## Active execution slice

### Slice 1: repo truth + deploy boundaries + first control-plane decomposition

**Slice 1 contributors**
- Main agent: tracker, integration, verification, site/runtime truth alignment, `internal/adminapi/*`
- Worker B: `Dockerfile`, `compose.yml`, `.github/workflows/ci.yml`
- Explorer: bundled plugin signature repair path

- [x] Split `internal/adminapi/server.go` into focused route/handler files without behavior change.
- [x] Fix Docker build pipeline issues in `Dockerfile`.
- [x] Separate bundled and mutable plugin paths in container/runtime config.
- [x] Add Docker image build coverage to CI.
- [x] Remove public-site claims that are not backed by repo implementation.
- [x] Repair bundled plugin signature verification or prove the exact missing signing asset.
- [x] Re-run targeted checks for all slice-1 changes.

## Architecture alignment backlog

### Phase 1: control-plane decomposition
- [x] Split `internal/adminapi/service.go` into focused files by domain.
- [x] Replace direct admin-to-bot hard dependency with bounded live Discord adapters.
- [x] Allow admin/control-plane startup without requiring a ready Discord bot instance.
- [x] Return precise control-plane errors for bot-only live guild operations when the gateway runtime is unavailable.

### Phase 2: command/module boundary rewrite
- [x] Introduce a single module contract that builtins and Lua plugins both satisfy.
- [x] Remove `internal/commands/api` as the umbrella command/runtime contract bucket.
- [x] Move builtin Discord command-tree registration builders out of builtin command definition packages.
- [x] Move the core builtin command family behind a runtime-owned handler adapter.
- [x] Move the admin `modules` builtin family behind a runtime-owned handler adapter.
- [x] Keep hybrid definition-backed and legacy builtin admin commands composed at runtime during the transition.
- [x] Move the admin `block` / `unblock` restriction family behind runtime-owned handler adapters.
- [x] Move the admin `plugins` builtin family behind runtime-owned handler adapters.
- [x] Remove the last builtin `Commands()` escape hatch so builtin command definition packages are definition-only.
- [x] Move builtin bridge locale flow to plain locale-code strings in transport-neutral packages.
- [x] Move the Discord-specific slash-command handler contract out of `internal/commands/api` and into the Discord runtime.
- [x] Separate command definition code from Discord transport/registration code.
- [x] Rename the Discord application-command runtime package to remove the `commands` / `commands` naming collision.
- [x] Replace the stale `internal/commands/api` helper package with `internal/commandtext`.
- [x] Remove the remaining command-layer alias churn (`cmdcore`, `cmdadmin`, `commandapi`) from command/runtime package names and imports.
- [x] Reduce alias-heavy package naming around command/runtime layers.

### Phase 3: plugin runtime decomposition
- [x] Split `internal/runtime/plugins/host.go` by loader, trust, registry, and execution responsibilities.
- [x] Move plugin typing/decoding boundaries out of raw cross-layer `any` flows.
  - [x] Move plugin autocomplete responses onto an encoded payload boundary across Lua VM -> plugin host -> Discord router.
  - [x] Move plugin interaction responses onto an encoded payload boundary across Lua VM -> plugin host -> Discord bridge.
  - [x] Move plugin automation responses and outbound Discord message payloads onto encoded boundaries across Lua VM -> plugin host -> Discord automation/executor paths.
- [x] Separate the Lua adapter layer from the Discord bridge layer more explicitly.
  - [x] Extract the shared plugin interaction contract out of `luaplugin` and remove the duplicate host-side Discord bridge interface.
  - [x] Move the remaining `luaplugin` Discord bridge contract out of `vm.go` into a dedicated bridge file.
  - [x] Replace loose host/VM `Discord` option wiring with an explicit `luaplugin.Bridge` dependency bundle in load/runtime construction paths.
  - [x] Move the Discord bridge implementation (`Executor`, `SlashInteraction`, executor-backed REST/files) out of `internal/runtime/discord/plugin` into a focused `internal/runtime/discord/pluginbridge` package.
  - [x] Move plugin route and automation bridge runtime ownership out of `internal/runtime/discord/plugin` into `internal/runtime/discord/pluginbridge`.
  - [x] Move host/runtime bridge wrapper ownership into `pluginhost` so `luaplugin.Bridge` is re-wrapped only at the Lua VM boundary.
- [x] Start moving plugin loading toward immutable bundle-oriented revisions instead of mutable directory assumptions.
  - [x] Split runtime plugin load metadata into entry dir vs bundle dir vs bundled-root ownership while keeping current discovery behavior unchanged.
  - [x] Add root-level bundle-state resolution so plugin roots can activate a separate bundle dir without exposing `plugin.json` at the entry root.
  - [x] Materialize marketplace installs and updates into versioned bundle dirs and switch the active bundle pointer instead of replacing the plugin root in place.
  - [x] Persist active bundle relative paths in `plugin_installs` and use that registry-backed path in marketplace/admin surfaces before filesystem fallback.
  - [x] Make the runtime plugin loader prefer stored `plugin_installs.bundle_relative_dir` over root manifests and pointer files, with filesystem fallback only for missing or invalid registry state.
  - [x] Make newly scaffolded manual plugins bundle-oriented from birth and make dashboard signing resolve/sign the active bundle dir instead of assuming a live root manifest.
  - [x] Expose real plugin provenance in the dashboard so marketplace-managed plugins are no longer labeled as generic user plugins.
  - [x] Move bundled first-party plugins onto the same root bundle-state + versioned-bundle layout, and make CLI/tests/docs resolve active bundle dirs instead of flat plugin roots.
  - [x] Remove flat plugin-root compatibility so runtime/admin/CLI/example plugin paths require root `.mamacord-bundle.json` state and active bundle resolution.

### Phase 4: unified scheduling
- [x] Introduce one scheduler/job runtime for reminders, plugin cron jobs, and future background tasks.
- [x] Migrate reminder polling onto the unified scheduler path.
- [x] Migrate plugin cron automation onto the unified scheduler path.

### Phase 5: scale-profile adapters
- [x] Move the operational storage path to Postgres-first runtime and deployment defaults.
  - [x] Add a real Postgres metadata/control-plane store slice for module states, trusted signers, marketplace sources/syncs, plugin installs, and trusted vendor records with parity tests.
  - [x] Remove `internal/app.App`'s concrete Postgres store type so backend selection can stop at storage wiring instead of the whole app type graph.
  - [x] Expand the Postgres adapter to satisfy the current runtime/control-plane store contract used by app startup, bot runtime, admin sessions, reminders, scheduler jobs, and plugin persistence paths.
  - [x] Fill the remaining in-repo Postgres persistence surfaces for `discord_oauth_tokens` and `plugin_oauth_grants`.
  - [x] Add a real pgx-backed Postgres opener plus translated `migrations/postgres/*` schema files.
  - [x] Dispatch app startup, CLI migrations, marketplace CLI boot, admin migration endpoints, and dashboard status reporting by configured storage backend instead of hardcoding Postgres.
  - [x] Make `mamacord init`, `mamacord doctor`, Compose, Docker defaults, and checked-in env examples surface Postgres as the primary storage path instead of a split-role-only add-on.
  - [x] Add a repo-native live-Postgres bootstrap integration harness plus a CI service-container job that targets `storagebootstrap.OpenRuntimeStore(...)`.
  - [x] Add live Postgres startup verification against a real Postgres instance.
    - [x] Add a live control-only `internal/app` startup integration test that boots MamaCord against a schema-scoped Postgres DSN and assert it reaches migrated runtime state before shutdown.
    - [x] Wire the Postgres CI service-container job to execute the live startup integration package set, including `internal/app`.
- [x] Add bundle/object-store adapter(s) suitable for shared deployments.
  - [x] Extract a dedicated `internal/bundles` repository seam and route runtime bundle resolution, marketplace materialization/removal, admin bundle listing/signing/scaffolding, and CLI active-bundle signing through it.
  - [x] Add a cached `bundles.Repository` implementation with separate bundle-store and worker-local active-cache paths, and wire bundle backend selection through app/runtime/admin/marketplace/CLI.
  - [x] Add an object-store-backed `bundles.Repository` on top of the cached artifact/cache contract, including repository-backed signature persistence for admin and CLI signing flows.
  - [x] Replace misleading admin/marketplace path-shaped response fields with stable `plugin_root` + `bundle_relative_dir` metadata, and move marketplace/admin bundle modification checks onto `bundles.Repository`.
  - [x] Move admin plugin summary inspection and runtime preferred-bundle loading onto shared `bundles.Inspect*` helpers so stored bundle overrides read manifests/signatures through the bundle repository and materialize object-store bundles into worker-local active cache paths.
- [x] Split runtime roles so control API, gateway, and scheduler can run independently.
  - [x] Add config-driven `control` / `gateway` / `scheduler` runtime-role selection with control-only boot no longer requiring `DISCORD_TOKEN`.
  - [x] Gate admin startup, Discord bot startup, command registration, gateway open, scheduler start, and readiness semantics by the configured runtime-role set.
  - [x] Surface the active runtime-role set through admin status, dashboard overview, docs, and `mamacord doctor`.
  - [x] Add split-role deployment truth to Compose/env/docs/CI, including separate `compose --profile split` role services, role-specific `doctor` smoke checks, and removal of container-side Postgres migration-path pinning so Postgres-backed split roles can use backend-selected migrations.
  - [x] Add boot-sequencing proof for control-only, gateway-only, scheduler-only, and combined role startup order in `internal/app`, plus direct dependency-flag coverage for gateway vs scheduler Discord runtime construction.

## Verification ledger

- [x] `GOCACHE=/private/tmp/go-build-cache go test ./internal/adminapi`
- [x] `GOCACHE=/private/tmp/go-build-cache go test ./internal/bundles`
- [x] `GOCACHE=/private/tmp/go-build-cache go test ./internal/storage/postgres`
- [x] `GOCACHE=/private/tmp/go-build-cache go test ./internal/storagebootstrap`
- [x] `GOCACHE=/private/tmp/go-build-cache go test ./internal/runtime/plugins`
- [x] `GOCACHE=/private/tmp/go-build-cache go test ./...`
- [x] `bun --cwd apps/dashboard run build`
- [x] `bun --cwd apps/site run build`
- [x] `docker build .`
