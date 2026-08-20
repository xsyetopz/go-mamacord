# Discord API Coverage (Plugins)

Mamacord plugins use a closed, typed host boundary. Starlark code never receives a Discord client or a Go object.

## Reads

Each read requires its matching manifest capability and counts against the callback host-call budget:

- user details
- guild member details
- guild details

Storage-backed reads cover user settings, check-ins, reminders, warnings, timezone normalization, and reminder planning under separate storage capabilities.

## Effects

Callbacks return ordered typed effects. Go validates the route, response state, Discord limits, and capabilities before the Discord adapter executes them.

Supported Discord effects are:

- interaction replies, response edits, source-message updates, modals, and autocomplete choices
- channel messages and direct messages
- member timeouts and nickname changes
- channel slowmode and message purge
- role create, edit, delete, add, and remove
- emoji create, edit, and delete
- sticker create, edit, and delete

Storage effects cover versioned guild KV, timezones, check-ins, reminders, warnings, and audit records.

## Authority

A plugin must request each capability in its strict v2 manifest. The host grants only the intersection with `config/permissions.json`. Missing capabilities deny both reads and effects.

Network access is available only through `ctx.http_get_json`. It requires `network.http`, an exact declared HTTPS hostname, public DNS results, TLS 1.2 or newer, no redirects, and a bounded JSON response.

## Extending the Surface

1. Add a closed typed operation or reader DTO in `internal/runtime/plugins/contract`.
2. Define its validation, cloning, capability, and invocation-scope rules.
3. Expose a bounded Starlark constructor or context method.
4. Implement the Discord or storage adapter without moving Disgo or infrastructure types into the contract.
5. Test denial, invalid values, limits, cancellation, and successful execution.
