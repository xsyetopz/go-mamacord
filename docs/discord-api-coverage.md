# Discord Features Available to Plugins

This page describes Mamacord's current **plugin** surface. It does not describe every feature used by the built-in bot, and it is not a map of the full Discord API.

A **capability** is a named host grant, such as `discord.messages`. A plugin must request it in `plugin.json`, and the Mamacord host must grant it in its [permissions policy](../config/permissions.json). The plugin receives only the intersection of those two sets. Discord's bot permissions and role hierarchy still apply.

Interaction replies, command registration, components, and modals do not use a `discord.*` capability. Mamacord still validates where and when they can be used.

## Data available to a handler

Mamacord gives each handler a typed `ctx` value. Depending on the trigger, it can contain the acting user, guild, channel, member, bot user, locale, slash-command path and options, string-select values, or submitted modal fields. This invocation data does not require a read capability.

Plugins can make these additional Discord reads:

| Starlark call | Capability | Result |
| --- | --- | --- |
| `ctx.get_user(user_id)` | `discord.users` | User identity, mention, avatar, banner, account creation time, and accent color |
| `ctx.get_member(user_id)` | `discord.members` | Member roles, join time, avatar, and banner in the current guild |
| `ctx.get_guild(guild_id)` | `discord.guilds` | Guild identity, owner, description, images, creation time, and counts for members, channels, roles, emojis, and stickers |

There are no on-demand reads for messages, channels, roles, emojis, stickers, invites, or threads. Channel, role, attachment, and similar values can still arrive as command options or interaction context.

## Commands and autocomplete

Plugins can register:

- slash commands, subcommands, and one level of subcommand groups;
- user context commands;
- message context commands;
- static choices and autocomplete for string, integer, and number options.

Slash options support `string`, `bool`, `int`, `float`, `user`, `channel`, `role`, `mentionable`, and `attachment`. Channel options can be limited to text, voice, category, announcement, stage, forum, or media channels.

Command access can use `guild_only`, `owner_only`, `has_permissions`, or a custom check. A top-level command can also set these Discord default member permissions:

`administrator`, `manage_guild`, `manage_roles`, `manage_expressions`, `create_expressions`, `manage_messages`, `manage_nicknames`, `manage_channels`, `kick_members`, `ban_members`, and `moderate_members`.

Autocomplete returns at most 25 choices whose value type matches the focused option.

**Current limitation:** Mamacord dispatches user and message context commands, but the selected user or message is not exposed on the Starlark `ctx` value.

## Replies, components, and modals

Interaction replies can contain text, embeds, and component rows. Embeds support titles, descriptions, HTTPS links, RGB colors, fields, authors, footers, images, and thumbnails. Allowed mentions are disabled at the Discord bridge.

| Feature | Supported behavior |
| --- | --- |
| Replies | Initial public or ephemeral reply; deferred-create response followed by an edit |
| Source-message updates | A component handler, or a modal opened from a component, can replace the source message's text, embeds, or components |
| Buttons | Primary, secondary, success, danger, and HTTPS link buttons; labels and disabled state |
| Select menus | String, user, role, mentionable, and channel selects; string choices and channel-kind filters |
| Modals | Open from a command or component; 1–5 short or paragraph text inputs; required fields and length limits |

Component and modal handlers must be declared by the plugin. Commands and components can open a modal before the interaction is deferred. A modal submission cannot open another modal.

**Current limitation:** `ctx.selected_values` exposes string-select values. Selected users, roles, mentionables, and channels from entity selects are not exposed to Starlark handlers.

## Discord effects

A handler returns ordered effects. Mamacord validates the effect, its capability, its guild or interaction scope, and Discord limits before calling Discord.

| Capability | Starlark effects | What they do |
| --- | --- | --- |
| `discord.messages` | `send_channel`, `send_dm` | Send a plain-text channel message or DM |
| `discord.messages` | `purge_messages` | Delete 1–100 messages using `all`, `before`, `after`, or `around` selection |
| `discord.members` | `timeout_member`, `set_nickname` | Set a future timeout; set or clear a guild nickname |
| `discord.channels` | `set_slowmode` | Set text-channel slowmode from 0 to 21,600 seconds |
| `discord.roles` | `create_role`, `edit_role`, `delete_role` | Manage role name, color, hoist, and mentionable state |
| `discord.roles` | `add_role`, `remove_role` | Add or remove a role from a guild member |
| `discord.emojis` | `create_emoji`, `edit_emoji`, `delete_emoji` | Create from an invocation attachment, rename, or delete a guild emoji |
| `discord.stickers` | `create_sticker`, `edit_sticker`, `delete_sticker` | Create from an invocation attachment, edit, or delete a guild sticker |

Emoji uploads accept GIF, JPEG, or PNG files up to 256 KiB and 320×320 pixels. Sticker uploads accept PNG, GIF, or APNG files up to 512 KiB and 320×320 pixels. The attachment must belong to the current command invocation; plugins cannot upload from an arbitrary URL.

The `reply`, `update`, `show_modal`, and `autocomplete_choices` effects are interaction responses, so they do not require a `discord.*` capability. They are valid only for matching interaction states. Mamacord does not expose interaction follow-up messages, arbitrary message edits, or arbitrary message deletion outside `purge_messages`.

## Gateway events and scheduled jobs

Plugins can subscribe to four Discord gateway events:

| Events | Manifest permission |
| --- | --- |
| `guild_member_join`, `guild_member_leave` | `automation.events.member_join_leave` |
| `guild_ban`, `guild_unban` | `automation.events.moderation` |

Event handlers receive the guild and affected user. They can return non-interaction effects from the table above when the required capability is granted.

Plugins can also declare scheduled jobs with `automation.jobs`. A job runs once per available guild on its schedule. Jobs are not Discord gateway events.

Automation permissions, like Discord capabilities, must be both requested by the plugin and granted by the host policy.

## Not available to plugins

The [plugin manifest schema](../schemas/plugin.schema.json) contains fields for reactions, threads, invites, and webhooks, but Mamacord rejects a plugin that requests any of them.

The plugin API also has no operations for:

- reactions, polls, or message history reads;
- creating, editing, or deleting channels and threads;
- permission overwrites, guild settings, bans, kicks, or member moves;
- invites, webhooks, voice or stage audio, presence, or arbitrary Discord REST calls;
- message file uploads, follow-up responses, or editing an arbitrary existing message.

Mamacord exposes a closed set of typed operations. A Discord endpoint that is not represented by a read, interaction response, event, or effect on this page is not available to plugins.
