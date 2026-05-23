# Contributing to go-mamacord

This project is a Go Discord bot with a Lua plugin host. Keep changes small, readable, and easy to review.

The stable internal name is `mamacord`. It appears in environment variables, database paths, plugin IDs, and Discord `custom_id` prefixes. Do not rename it casually.

## Before Starting

- Read [README.md](README.md) for the shortest “get running” guide.
- Read [docs/reference.md](docs/reference.md) for the longer runtime/plugin/module details.
- Check existing issues or pull requests before starting overlapping work.
- If your change touches plugins, schemas, or localization, inspect the shipped example in `examples/plugins/example/`.

## Standard Workflow

1. Fork the repository.
2. Clone your fork: `git clone https://github.com/<your-user>/go-mamacord.git`
3. Create a focused branch: `git checkout -b feat/<short-description>`
4. Make the smallest change that solves the problem.
5. Format and test the affected code.
6. Commit with a message that explains why the change exists.
7. Open a pull request that explains the behavior change and how you verified it.

## Repository Shape

The main code paths are:

- `cmd/mamacord/` for the process entrypoint.
- `internal/app/` for application wiring.
- `internal/runtime/discord/` for Discord transport and runtime behavior.
- `internal/commands/` for built-in kernel commands and shared command contracts.
- `internal/runtime/plugins/` for plugin discovery, policy, signing, and dispatch.
- `internal/runtime/plugins/lua/` for the embedded Lua runtime and SDK bridge.
- `sdk/lua/` for Lua editor stubs.
- `examples/plugins/` for shipped sample plugins.
- `plugins/` for runtime-loaded plugins.

Keep those boundaries intact. Avoid mixing Discord transport concerns, feature logic, and plugin host internals in the same change unless the behavior truly crosses those layers.

## Coding Guidelines

### General

- Prefer simple, explicit code over abstractions used only once.
- Keep functions focused. If a function has two separate reasons to change, split it.
- Do not rename broad project concepts casually. Names like plugin ID, command name, locale ID, and permission key are part of the external contract.
- Remove dead code and commented-out experiments before submitting.

### Go

- Run `gofmt` on every edited Go file.
- Prefer clear package boundaries over catch-all files or generic helpers.
- Keep error messages specific and actionable.
- Avoid hidden behavior in refactors. Structural changes should preserve behavior unless the pull request explicitly changes behavior.

### Lua Plugins

- New plugin examples and docs should use the `bot` API, not legacy callback-style globals.
- Prefer the descriptor model: `bot.plugin`, `bot.command`, typed `ctx`, and `bot.ui` or `bot.effects`.
- Keep example plugins beginner-friendly. A new contributor should be able to understand the sample without knowing Discord payload internals.

### Comments

- Comment only when the reason for the code is not obvious from the code itself.
- Avoid section-divider comments and boilerplate narration.
- Public docs should match actual repo behavior, paths, and command names.

## Testing

Run the narrowest useful test first, then the broader suite if your change crosses multiple areas.

### Common Commands

```bash
# Run the whole Go test suite
go test ./...

# Run a focused package test
go test ./internal/runtime/plugins/...

# Format edited Go files
gofmt -w ./internal/...
```

If you use `golangci-lint` locally, run it on the code you touched or on the full repo if practical.

### When Touching Schemas, Config, or Plugin Layout

Run the coverage audit script:

```bash
./scripts/audit_coverage.sh
```

That default audit checks the shipped JSON files and schemas for required metadata like `"$schema"` and `"$id"`.

## Pull Request Checklist

- [ ] The change is scoped tightly and does not mix unrelated cleanup.
- [ ] Edited Go files are formatted.
- [ ] Relevant `go test` commands pass.
- [ ] Docs, examples, or schemas were updated if the public contract changed.
- [ ] Plugin-related changes keep the `bot` authoring model and shipped example coherent.
- [ ] Commit messages and the PR description explain the motivation and the verification performed.

## Reporting Issues

When filing a bug, include:

- Steps to reproduce.
- Expected behavior.
- Actual behavior.
- Relevant logs, screenshots, or command output.
- Environment details such as OS, architecture, and whether you ran the bot directly or through Docker.

For plugin issues, include the relevant `plugin.json`, the route being exercised, and any permission or signing context that matters.

## Development Setup

### Prerequisites

- Go 1.26.3 or newer
- Git
- A Discord bot token for local runtime testing

### Typical Commands

```bash
# Run the bot locally
go run ./cmd/mamacord

# Run with Docker
docker compose up --build
```

Use `.env.local.example` as the starting point for local bot/API development,
and `apps/dashboard/.env.local.example` for local dashboard development.

## Using AI Assistants

AI-generated changes are allowed, but contributors remain responsible for the result.

- Read the surrounding code before accepting generated edits.
- Verify generated behavior with tests or direct inspection.
- Do not submit generated text that still refers to another project, language, or architecture.

## Questions and Support

Open an issue or pull request discussion if direction is unclear. Keep technical decisions in public where possible so the next contributor can follow the reasoning.

## Code of Conduct

All contributors must follow the [Code of Conduct](CODE_OF_CONDUCT.md).
