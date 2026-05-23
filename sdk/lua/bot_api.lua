---@meta

---@class MamaCordGuildRef
---@field id string

---@class MamaCordChannelRef
---@field id string

---@class MamaCordUserRef
---@field id string

---@class MamaCordAttachmentRef
---@field id string
---@field filename string
---@field url string
---@field size integer
---@field width? integer
---@field height? integer
---@field content_type? string

---@class MamaCordPluginRef
---@field id string

---@class MamaCordScopedStore
---@field get fun(key: string): (any|nil, boolean)
---@field put fun(key: string, value: any): boolean
---@field del fun(key: string): boolean
---@field get_json fun(key: string): (string|nil, boolean)
---@field put_json fun(key: string, value_json: string): boolean

---@class MamaCordCommandContext
---@field name string
---@field kind 'slash'|'user'|'message'
---@field group string
---@field subcommand string
---@field args table<string, any>
---@field resolved table<string, table>

---@class MamaCordAutocompleteContext
---@field command string
---@field group string
---@field subcommand string
---@field option string
---@field value any

---@class MamaCordTargetContext
---@field user? MamaCordUser
---@field member? MamaCordMember
---@field message? MamaCordMessageInfo

---@class MamaCordComponentContext
---@field id string
---@field kind string
---@field values any[]|nil

---@class MamaCordModalContext
---@field id string
---@field fields table<string, any>

---@class MamaCordEventContext
---@field name string

---@class MamaCordJobContext
---@field id string

---@class MamaCordRouteContext
---@field guild_id string
---@field channel_id string
---@field user_id string
---@field locale string
---@field guild MamaCordGuildRef
---@field channel MamaCordChannelRef
---@field user MamaCordUserRef
---@field plugin MamaCordPluginRef
---@field store MamaCordScopedStore
---@field options table<string, any>
---@field args table<string, any>|nil
---@field command MamaCordCommandContext|nil
---@field component MamaCordComponentContext|nil
---@field modal MamaCordModalContext|nil
---@field event MamaCordEventContext|nil
---@field job MamaCordJobContext|nil
---@field target MamaCordTargetContext|nil
---@field autocomplete MamaCordAutocompleteContext|nil
---@field bot MamaCordAPI

---@class MamaCordPresent
---@field kind? 'info'|'success'|'warning'|'error'|'ok'|'warn'|'err'
---@field title? string
---@field body? string
---@field fields? { name: string, value: string, inline?: boolean }[]

---@class MamaCordButton
---@field type 'button'
---@field id string
---@field label? string
---@field style? 'primary'|'secondary'|'success'|'danger'|'link'
---@field url? string
---@field disabled? boolean

---@class MamaCordStringSelectOption
---@field label string
---@field value string
---@field description? string

---@class MamaCordStringSelect
---@field type 'string_select'
---@field id string
---@field placeholder? string
---@field min_values? integer
---@field max_values? integer
---@field disabled? boolean
---@field options MamaCordStringSelectOption[]

---@class MamaCordUserSelect
---@field type 'user_select'
---@field id string
---@field placeholder? string
---@field min_values? integer
---@field max_values? integer
---@field disabled? boolean

---@class MamaCordRoleSelect
---@field type 'role_select'
---@field id string
---@field placeholder? string
---@field min_values? integer
---@field max_values? integer
---@field disabled? boolean

---@class MamaCordMentionableSelect
---@field type 'mentionable_select'
---@field id string
---@field placeholder? string
---@field min_values? integer
---@field max_values? integer
---@field disabled? boolean

---@class MamaCordChannelSelect
---@field type 'channel_select'
---@field id string
---@field placeholder? string
---@field min_values? integer
---@field max_values? integer
---@field disabled? boolean
---@field channel_types? integer[]

---@alias MamaCordComponent MamaCordButton|MamaCordStringSelect|MamaCordUserSelect|MamaCordRoleSelect|MamaCordMentionableSelect|MamaCordChannelSelect

---@class MamaCordEmbedField
---@field name string
---@field value string
---@field inline? boolean

---@class MamaCordEmbed
---@field title? string
---@field description? string
---@field url? string
---@field color? integer
---@field image_url? string
---@field thumbnail_url? string
---@field footer? string|{ text: string, icon_url?: string }
---@field author? { name: string, url?: string, icon_url?: string }
---@field fields? MamaCordEmbedField[]

---@class MamaCordModalField
---@field id string
---@field label string
---@field description? string
---@field style? 'short'|'paragraph'
---@field required? boolean
---@field placeholder? string
---@field value? string
---@field min_length? integer
---@field max_length? integer

---@class MamaCordResponseBase
---@field ephemeral? boolean
---@field content? string
---@field embeds? MamaCordEmbed[]
---@field components? MamaCordComponent[][]

---@class MamaCordMessageResponse: MamaCordResponseBase
---@field type? 'message'|'update'
---@field present? MamaCordPresent

---@class MamaCordModalResponse
---@field type 'modal'
---@field id string
---@field title string
---@field components MamaCordModalField[]

---@alias MamaCordResponse MamaCordMessageResponse|MamaCordModalResponse

---@class MamaCordCommandChoice
---@field name string
---@field value string|number|boolean

---@class MamaCordCommandOption
---@field name string
---@field type 'string'|'bool'|'int'|'float'|'user'|'channel'|'role'|'mentionable'|'attachment'
---@field description string
---@field description_id? string
---@field required? boolean
---@field autocomplete? string
---@field choices? MamaCordCommandChoice[]
---@field min_value? number
---@field max_value? number
---@field min_length? integer
---@field max_length? integer
---@field channel_types? integer[]

---@class MamaCordSubcommand
---@field name string
---@field description string
---@field description_id? string
---@field ephemeral? boolean
---@field options? MamaCordCommandOption[]

---@class MamaCordCommandGroup
---@field name string
---@field description string
---@field description_id? string
---@field subcommands MamaCordSubcommand[]

---@class MamaCordCommandRoute
---@field type? 'slash'
---@field name string
---@field description string
---@field description_id? string
---@field ephemeral? boolean
---@field default_member_permissions? string[]
---@field options? MamaCordCommandOption[]
---@field subcommands? MamaCordSubcommand[]
---@field groups? MamaCordCommandGroup[]
---@field run fun(ctx: MamaCordRouteContext): MamaCordResponse|nil

---@class MamaCordUserCommandRoute
---@field type 'user'
---@field name string
---@field default_member_permissions? string[]
---@field run fun(ctx: MamaCordRouteContext): MamaCordResponse|nil

---@class MamaCordMessageCommandRoute
---@field type 'message'
---@field name string
---@field default_member_permissions? string[]
---@field run fun(ctx: MamaCordRouteContext): MamaCordResponse|nil

---@class MamaCordJobRoute
---@field id string
---@field schedule string
---@field run fun(ctx: MamaCordRouteContext): table|nil

---@class MamaCordPluginDefinition
---@field commands? MamaCordCommandRoute[]
---@field user_commands? MamaCordUserCommandRoute[]
---@field message_commands? MamaCordMessageCommandRoute[]
---@field autocompletes? table<string, fun(ctx: MamaCordRouteContext): MamaCordCommandChoice[]|{ choices: MamaCordCommandChoice[] }|nil>
---@field components? table<string, fun(ctx: MamaCordRouteContext): MamaCordResponse|nil>
---@field modals? table<string, fun(ctx: MamaCordRouteContext): MamaCordResponse|nil>
---@field events? table<string, fun(ctx: MamaCordRouteContext): table|nil>
---@field jobs? MamaCordJobRoute[]

---@class MamaCordLogAPI
---@field info fun(msg: string)

---@class MamaCordI18nAPI
---@field t fun(message_id: string, data: table|nil, plural_count: any|nil): string

---@class MamaCordStoreAPI
---@field get fun(guild_id: string, key: string): (any|nil, boolean)
---@field put fun(guild_id: string, key: string, value: any): boolean
---@field del fun(guild_id: string, key: string): boolean
---@field get_json fun(guild_id: string, key: string): (string|nil, boolean)
---@field put_json fun(guild_id: string, key: string, value_json: string): boolean

---@class MamaCordOptionAPI
---@field string fun(name: string, spec: table): MamaCordCommandOption
---@field bool fun(name: string, spec: table): MamaCordCommandOption
---@field int fun(name: string, spec: table): MamaCordCommandOption
---@field float fun(name: string, spec: table): MamaCordCommandOption
---@field user fun(name: string, spec: table): MamaCordCommandOption
---@field channel fun(name: string, spec: table): MamaCordCommandOption
---@field role fun(name: string, spec: table): MamaCordCommandOption
---@field mentionable fun(name: string, spec: table): MamaCordCommandOption
---@field attachment fun(name: string, spec: table): MamaCordCommandOption

---@class MamaCordUIAPI
---@field reply fun(spec: table): MamaCordMessageResponse
---@field defer fun(spec?: { ephemeral?: boolean }): (boolean, string|nil)
---@field update fun(spec: table): MamaCordMessageResponse
---@field modal fun(id: string, spec: table): MamaCordModalResponse
---@field present fun(spec: table): MamaCordMessageResponse
---@field button fun(id: string, spec: table): MamaCordButton
---@field choice fun(name: string, value: string|number|boolean): MamaCordCommandChoice
---@field choices fun(list: MamaCordCommandChoice[]): MamaCordCommandChoice[]
---@field string_option fun(label: string, value: string, spec?: table): MamaCordStringSelectOption
---@field string_select fun(id: string, spec: table): MamaCordStringSelect
---@field text_input fun(id: string, spec: table): MamaCordModalField

---@class MamaCordEffectsAPI
---Automation-only effects for event/job handlers.
---@field send_channel fun(spec: { channel_id?: string, message: MamaCordResponse|string }): table
---@field send_dm fun(spec: { user_id?: string, message: MamaCordResponse|string }): table
---@field timeout_member fun(spec: { guild_id?: string, user_id?: string, until_unix: integer }): table

---@class MamaCordDiscordSendResult
---@field message_id string
---@field channel_id string
---@field user_id? string

---@class MamaCordRole
---@field id string|integer
---@field name string
---@field mention string
---@field color integer
---@field hoist boolean
---@field mentionable boolean
---@field position integer
---@field managed boolean
---@field permissions integer
---@field created_at integer

---@class MamaCordUser
---@field id string|integer
---@field username string
---@field display_name string
---@field mention string
---@field bot boolean
---@field system boolean
---@field accent_color integer
---@field avatar_url string
---@field banner_url string
---@field created_at integer

---@class MamaCordMember
---@field user_id string|integer
---@field guild_id string|integer
---@field joined_at integer
---@field role_ids (string|integer)[]
---@field avatar_url string
---@field banner_url string

---@class MamaCordGuild
---@field id string|integer
---@field name string
---@field description string
---@field owner_id string|integer
---@field roles_count integer
---@field emojis_count integer
---@field stickers_count integer
---@field member_count integer
---@field channels_count integer
---@field icon_url string
---@field banner_url string
---@field created_at integer

---@class MamaCordChannel
---@field id string|integer
---@field name string
---@field mention string
---@field type string
---@field parent_id? string|integer
---@field permissions integer
---@field created_at integer

---@class MamaCordMessageInfo
---@field id string|integer
---@field channel_id string|integer
---@field author_id string|integer
---@field content string
---@field created_at integer
---@field edited_at? integer
---@field pinned? boolean

---@class MamaCordDiscordMessagesAPI
---@field get fun(spec: { channel_id?: string|integer, message_id: string|integer }): (MamaCordMessageInfo|nil, string|nil)
---@field list fun(spec: { channel_id?: string|integer, around_message_id?: string|integer, before_message_id?: string|integer, after_message_id?: string|integer, limit: integer }): (MamaCordMessageInfo[]|nil, string|nil)
---@field delete fun(spec: { channel_id?: string|integer, message_id: string|integer }): (boolean, string|nil)
---@field bulk_delete fun(spec: { channel_id?: string|integer, message_ids: (string|integer)[] }): ({ deleted_count: integer }|nil, string|nil)
---@field purge fun(spec: { channel_id?: string|integer, mode: "all"|"before"|"after"|"around", anchor_message_id?: string|integer, count: integer }): ({ deleted_count: integer }|nil, string|nil)
---@field crosspost fun(spec: { channel_id?: string|integer, message_id: string|integer }): (MamaCordMessageInfo|nil, string|nil)
---@field pin fun(spec: { channel_id?: string|integer, message_id: string|integer }): (boolean, string|nil)
---@field unpin fun(spec: { channel_id?: string|integer, message_id: string|integer }): (boolean, string|nil)

---@class MamaCordDiscordReactionsAPI
---@field list fun(spec: { channel_id?: string|integer, message_id: string|integer, emoji: string, after_user_id?: string|integer, limit?: integer }): (MamaCordUser[]|nil, string|nil)
---@field add fun(spec: { channel_id?: string|integer, message_id: string|integer, emoji: string }): (boolean, string|nil)
---@field remove_own fun(spec: { channel_id?: string|integer, message_id: string|integer, emoji: string }): (boolean, string|nil)
---@field remove_user fun(spec: { channel_id?: string|integer, message_id: string|integer, user_id?: string|integer, emoji: string }): (boolean, string|nil)
---@field clear fun(spec: { channel_id?: string|integer, message_id: string|integer }): (boolean, string|nil)
---@field clear_for_emoji fun(spec: { channel_id?: string|integer, message_id: string|integer, emoji: string }): (boolean, string|nil)

---@class MamaCordEmoji
---@field id string|integer
---@field name string

---@class MamaCordSticker
---@field id string|integer
---@field name string

---@class MamaCordDiscordUsersAPI
---@field self fun(): (MamaCordUser|nil, string|nil)
---@field get fun(spec?: { user_id?: string|integer }): (MamaCordUser|nil, string|nil)

---@class MamaCordDiscordGuildsAPI
---@field get fun(spec?: { guild_id?: string|integer }): (MamaCordGuild|nil, string|nil)
---@field list_invites fun(spec: { guild_id?: string|integer }): (MamaCordInvite[]|nil, string|nil)

---@class MamaCordDiscordChannelsAPI
---@field get fun(spec?: { channel_id?: string|integer }): (MamaCordChannel|nil, string|nil)
---@field create fun(spec: { guild_id?: string|integer, name: string, type?: string, topic?: string, parent_id?: string|integer, nsfw?: boolean, slowmode?: integer, position?: integer, bitrate?: integer, user_limit?: integer }): (MamaCordChannel|nil, string|nil)
---@field edit fun(spec: { channel_id?: string|integer, name?: string, topic?: string, parent_id?: string|integer, nsfw?: boolean, slowmode?: integer, position?: integer, bitrate?: integer, user_limit?: integer }): (MamaCordChannel|nil, string|nil)
---@field delete fun(spec: { channel_id?: string|integer }): (boolean, string|nil)
---@field set_slowmode fun(spec: { channel_id?: string|integer, seconds: integer }): (boolean, string|nil)
---@field set_overwrite fun(spec: { channel_id?: string|integer, target_id: string|integer, target_type: 'role'|'member'|'user', allow?: integer, deny?: integer }): (boolean, string|nil)
---@field delete_overwrite fun(spec: { channel_id?: string|integer, target_id: string|integer }): (boolean, string|nil)
---@field list_invites fun(spec: { channel_id?: string|integer }): (MamaCordInvite[]|nil, string|nil)
---@field list_webhooks fun(spec: { channel_id?: string|integer }): (MamaCordWebhook[]|nil, string|nil)

---@class MamaCordDiscordMembersAPI
---@field get fun(spec?: { guild_id?: string|integer, user_id?: string|integer }): (MamaCordMember|nil, string|nil)
---@field timeout fun(spec: { guild_id?: string, user_id?: string, until_unix: integer }): (boolean, string|nil)
---@field set_nickname fun(spec: { guild_id?: string|integer, user_id?: string|integer, nickname?: string }): (boolean, string|nil)

---@class MamaCordDiscordRolesAPI
---@field get fun(spec: { guild_id?: string|integer, role_id: string|integer }): (MamaCordRole|nil, string|nil)
---@field create fun(spec: { guild_id?: string|integer, name: string, color?: integer, hoist?: boolean, mentionable?: boolean }): (MamaCordRole|nil, string|nil)
---@field edit fun(spec: { guild_id?: string|integer, role_id: string|integer, name?: string, color?: integer, hoist?: boolean, mentionable?: boolean }): (MamaCordRole|nil, string|nil)
---@field delete fun(spec: { guild_id?: string|integer, role_id: string|integer }): (boolean, string|nil)
---@field add_to_member fun(spec: { guild_id?: string|integer, user_id?: string|integer, role_id: string|integer }): (boolean, string|nil)
---@field remove_from_member fun(spec: { guild_id?: string|integer, user_id?: string|integer, role_id: string|integer }): (boolean, string|nil)

---@class MamaCordThread
---@field id string|integer
---@field guild_id string|integer
---@field parent_id string|integer
---@field name string
---@field mention string
---@field type string
---@field archived boolean
---@field locked boolean
---@field auto_archive_duration integer
---@field created_at integer

---@class MamaCordDiscordThreadsAPI
---@field create_from_message fun(spec: { channel_id?: string|integer, message_id: string|integer, name: string, auto_archive_duration?: integer, slowmode?: integer }): (MamaCordThread|nil, string|nil)
---@field create_in_channel fun(spec: { channel_id?: string|integer, name: string, type?: string, auto_archive_duration?: integer, invitable?: boolean }): (MamaCordThread|nil, string|nil)
---@field join fun(spec: { thread_id?: string|integer }): (boolean, string|nil)
---@field leave fun(spec: { thread_id?: string|integer }): (boolean, string|nil)
---@field add_member fun(spec: { thread_id?: string|integer, user_id?: string|integer }): (boolean, string|nil)
---@field remove_member fun(spec: { thread_id?: string|integer, user_id?: string|integer }): (boolean, string|nil)
---@field update fun(spec: { thread_id?: string|integer, name?: string, archived?: boolean, locked?: boolean, invitable?: boolean, auto_archive_duration?: integer, slowmode?: integer }): (MamaCordThread|nil, string|nil)

---@class MamaCordInvite
---@field code string
---@field url string
---@field guild_id string|integer
---@field channel_id string|integer
---@field inviter_id string|integer
---@field max_age integer
---@field max_uses integer
---@field uses integer
---@field temporary boolean
---@field created_at integer

---@class MamaCordDiscordInvitesAPI
---@field create fun(spec: { channel_id?: string|integer, max_age?: integer, max_uses?: integer, temporary?: boolean, unique?: boolean }): (MamaCordInvite|nil, string|nil)
---@field get fun(spec: { code: string }): (MamaCordInvite|nil, string|nil)
---@field delete fun(spec: { code: string }): (boolean, string|nil)
---@field list_channel fun(spec: { channel_id?: string|integer }): (MamaCordInvite[]|nil, string|nil)
---@field list_guild fun(spec: { guild_id?: string|integer }): (MamaCordInvite[]|nil, string|nil)

---@class MamaCordWebhook
---@field id string|integer
---@field guild_id string|integer
---@field channel_id string|integer
---@field application_id string|integer
---@field name string
---@field token string
---@field url string

---@class MamaCordDiscordWebhooksAPI
---@field create fun(spec: { channel_id?: string|integer, name: string }): (MamaCordWebhook|nil, string|nil)
---@field get fun(spec: { webhook_id: string|integer }): (MamaCordWebhook|nil, string|nil)
---@field list_channel fun(spec: { channel_id?: string|integer }): (MamaCordWebhook[]|nil, string|nil)
---@field edit fun(spec: { webhook_id: string|integer, name?: string, channel_id?: string|integer }): (MamaCordWebhook|nil, string|nil)
---@field delete fun(spec: { webhook_id: string|integer }): (boolean, string|nil)
---@field execute fun(spec: { webhook_id: string|integer, token: string }, message: MamaCordResponse|string): (MamaCordDiscordSendResult|nil, string|nil)

---@class MamaCordDiscordEmojisAPI
---@field create fun(spec: { guild_id?: string|integer, name: string, file: MamaCordAttachmentRef }): (MamaCordEmoji|nil, string|nil)
---@field edit fun(spec: { guild_id?: string|integer, emoji: string, name: string }): (MamaCordEmoji|nil, string|nil)
---@field delete fun(spec: { guild_id?: string|integer, emoji: string }): (boolean, string|nil)

---@class MamaCordDiscordStickersAPI
---@field create fun(spec: { guild_id?: string|integer, name: string, description?: string, emoji_tag: string, file: MamaCordAttachmentRef }): (MamaCordSticker|nil, string|nil)
---@field edit fun(spec: { guild_id?: string|integer, id: string, name: string, description?: string }): (MamaCordSticker|nil, string|nil)
---@field delete fun(spec: { guild_id?: string|integer, id: string }): (boolean, string|nil)

---@class MamaCordDiscordAPI
---@field self_user fun(): (MamaCordUser|nil, string|nil)
---@field get_user fun(spec?: { user_id?: string|integer }): (MamaCordUser|nil, string|nil)
---@field get_member fun(spec?: { guild_id?: string|integer, user_id?: string|integer }): (MamaCordMember|nil, string|nil)
---@field get_guild fun(spec?: { guild_id?: string|integer }): (MamaCordGuild|nil, string|nil)
---@field get_role fun(spec: { guild_id?: string|integer, role_id: string|integer }): (MamaCordRole|nil, string|nil)
---@field get_channel fun(spec?: { channel_id?: string|integer }): (MamaCordChannel|nil, string|nil)
---@field get_message fun(spec: { channel_id?: string|integer, message_id: string|integer }): (MamaCordMessageInfo|nil, string|nil)
---@field send_dm fun(spec: { user_id?: string, message: MamaCordResponse|string }): (MamaCordDiscordSendResult|nil, string|nil)
---@field send_channel fun(spec: { channel_id?: string, message: MamaCordResponse|string }): (MamaCordDiscordSendResult|nil, string|nil)
---@field timeout_member fun(spec: { guild_id?: string, user_id?: string, until_unix: integer }): (boolean, string|nil)
---@field set_slowmode fun(spec: { channel_id?: string|integer, seconds: integer }): (boolean, string|nil)
---@field set_nickname fun(spec: { guild_id?: string|integer, user_id?: string|integer, nickname?: string }): (boolean, string|nil)
---@field create_role fun(spec: { guild_id?: string|integer, name: string, color?: integer, hoist?: boolean, mentionable?: boolean }): (MamaCordRole|nil, string|nil)
---@field edit_role fun(spec: { guild_id?: string|integer, role_id: string|integer, name?: string, color?: integer, hoist?: boolean, mentionable?: boolean }): (MamaCordRole|nil, string|nil)
---@field delete_role fun(spec: { guild_id?: string|integer, role_id: string|integer }): (boolean, string|nil)
---@field add_role fun(spec: { guild_id?: string|integer, user_id?: string|integer, role_id: string|integer }): (boolean, string|nil)
---@field remove_role fun(spec: { guild_id?: string|integer, user_id?: string|integer, role_id: string|integer }): (boolean, string|nil)
---@field list_messages fun(spec: { channel_id?: string|integer, around_message_id?: string|integer, before_message_id?: string|integer, after_message_id?: string|integer, limit: integer }): (MamaCordMessageInfo[]|nil, string|nil)
---@field delete_message fun(spec: { channel_id?: string|integer, message_id: string|integer }): (boolean, string|nil)
---@field bulk_delete_messages fun(spec: { channel_id?: string|integer, message_ids: (string|integer)[] }): ({ deleted_count: integer }|nil, string|nil)
---@field purge_messages fun(spec: { channel_id?: string|integer, mode: "all"|"before"|"after"|"around", anchor_message_id?: string|integer, count: integer }): ({ deleted_count: integer }|nil, string|nil)
---@field crosspost_message fun(spec: { channel_id?: string|integer, message_id: string|integer }): (MamaCordMessageInfo|nil, string|nil)
---@field pin_message fun(spec: { channel_id?: string|integer, message_id: string|integer }): (boolean, string|nil)
---@field unpin_message fun(spec: { channel_id?: string|integer, message_id: string|integer }): (boolean, string|nil)
---@field get_reactions fun(spec: { channel_id?: string|integer, message_id: string|integer, emoji: string, after_user_id?: string|integer, limit?: integer }): (MamaCordUser[]|nil, string|nil)
---@field add_reaction fun(spec: { channel_id?: string|integer, message_id: string|integer, emoji: string }): (boolean, string|nil)
---@field remove_own_reaction fun(spec: { channel_id?: string|integer, message_id: string|integer, emoji: string }): (boolean, string|nil)
---@field remove_user_reaction fun(spec: { channel_id?: string|integer, message_id: string|integer, user_id?: string|integer, emoji: string }): (boolean, string|nil)
---@field clear_reactions fun(spec: { channel_id?: string|integer, message_id: string|integer }): (boolean, string|nil)
---@field clear_reactions_for_emoji fun(spec: { channel_id?: string|integer, message_id: string|integer, emoji: string }): (boolean, string|nil)
---@field messages MamaCordDiscordMessagesAPI
---@field reactions MamaCordDiscordReactionsAPI
---@field users MamaCordDiscordUsersAPI
---@field guilds MamaCordDiscordGuildsAPI
---@field channels MamaCordDiscordChannelsAPI
---@field members MamaCordDiscordMembersAPI
---@field roles MamaCordDiscordRolesAPI
---@field threads MamaCordDiscordThreadsAPI
---@field invites MamaCordDiscordInvitesAPI
---@field webhooks MamaCordDiscordWebhooksAPI
---@field emojis MamaCordDiscordEmojisAPI
---@field stickers MamaCordDiscordStickersAPI
---@field create_emoji fun(spec: { guild_id?: string|integer, name: string, file: MamaCordAttachmentRef }): (MamaCordEmoji|nil, string|nil)
---@field edit_emoji fun(spec: { guild_id?: string|integer, emoji: string, name: string }): (MamaCordEmoji|nil, string|nil)
---@field delete_emoji fun(spec: { guild_id?: string|integer, emoji: string }): (boolean, string|nil)
---@field create_sticker fun(spec: { guild_id?: string|integer, name: string, description?: string, emoji_tag: string, file: MamaCordAttachmentRef }): (MamaCordSticker|nil, string|nil)
---@field edit_sticker fun(spec: { guild_id?: string|integer, id: string, name: string, description?: string }): (MamaCordSticker|nil, string|nil)
---@field delete_sticker fun(spec: { guild_id?: string|integer, id: string }): (boolean, string|nil)

---@class MamaCordRandomAPI
---@field int fun(min: integer, max: integer): integer
---@field choice fun(list: any[]): any

---@class MamaCordTimeAPI
---@field unix fun(): integer

---@class MamaCordRuntimeAPI
---@field build_info fun(): { version: string, description: string, repository: string, mascot_image_url: string, developer_url: string, support_server_url: string }

---@class MamaCordHTTPResponse
---@field status integer
---@field body string
---@field headers table<string, string>

---@class MamaCordHTTPAPI
---@field get fun(spec: { url: string, headers?: table<string, string>, max_bytes?: integer }): MamaCordHTTPResponse
---@field get_json fun(spec: { url: string, headers?: table<string, string>, max_bytes?: integer }): any

---@class MamaCordUserSettings
---@field user_id integer
---@field timezone string
---@field dm_channel_id string
---@field created_at integer
---@field updated_at integer

---@class MamaCordUserSettingsAPI
---@field normalize_timezone fun(timezone: string): string|nil
---@field get fun(user_id?: string|integer): (MamaCordUserSettings|nil, boolean)
---@field set_timezone fun(user_id: string|integer, timezone: string): string
---@field clear_timezone fun(user_id?: string|integer): boolean

---@class MamaCordCheckIn
---@field id string
---@field user_id integer
---@field mood integer
---@field created_at integer

---@class MamaCordCheckInsAPI
---@field create fun(spec: { user_id?: string|integer, mood: integer, created_at?: integer }): MamaCordCheckIn
---@field list fun(user_id?: string|integer, limit?: integer): MamaCordCheckIn[]

---@class MamaCordReminder
---@field id string
---@field user_id integer
---@field schedule string
---@field kind string
---@field note string
---@field delivery string
---@field guild_id string
---@field channel_id string
---@field enabled boolean
---@field next_run_at integer
---@field last_run_at integer|nil
---@field failure_count integer
---@field created_at integer
---@field updated_at integer

---@class MamaCordReminderPlan
---@field schedule string
---@field next_run_at integer

---@class MamaCordRemindersAPI
---@field plan fun(spec: { user_id?: string|integer, schedule: string }): MamaCordReminderPlan|nil
---@field create fun(spec: { user_id?: string|integer, schedule: string, kind: string, note?: string, delivery?: string, guild_id?: string|integer, channel_id?: string|integer }): MamaCordReminder|nil
---@field list fun(user_id?: string|integer, limit?: integer): MamaCordReminder[]
---@field delete fun(user_id: string|integer, reminder_id: string): boolean

---@class MamaCordWarning
---@field id string
---@field guild_id integer
---@field user_id integer
---@field moderator_id integer
---@field reason string
---@field created_at integer

---@class MamaCordWarningsAPI
---@field count fun(guild_id?: string|integer, user_id?: string|integer): integer
---@field list fun(guild_id?: string|integer, user_id?: string|integer, limit?: integer): MamaCordWarning[]
---@field create fun(spec: { id?: string, guild_id?: string|integer, user_id?: string|integer, moderator_id?: string|integer, reason: string, created_at?: integer }): MamaCordWarning
---@field delete fun(warning_id: string): boolean

---@class MamaCordAuditAPI
---@field append fun(spec: { guild_id?: string|integer, actor_id?: string|integer, action: string, target_type?: 'user'|'guild', target_id?: string|integer, created_at?: integer, meta_json?: string }): boolean

---@class MamaCordAPI
---@field log MamaCordLogAPI
---@field i18n MamaCordI18nAPI
---@field store MamaCordStoreAPI
---@field usersettings MamaCordUserSettingsAPI
---@field checkins MamaCordCheckInsAPI
---@field reminders MamaCordRemindersAPI
---@field warnings MamaCordWarningsAPI
---@field audit MamaCordAuditAPI
---@field option MamaCordOptionAPI
---@field ui MamaCordUIAPI
---@field effects MamaCordEffectsAPI
---@field discord MamaCordDiscordAPI
---@field runtime MamaCordRuntimeAPI
---@field random MamaCordRandomAPI
---@field time MamaCordTimeAPI
---@field http MamaCordHTTPAPI
---@field plugin fun(spec: MamaCordPluginDefinition): MamaCordPluginDefinition
---@field command fun(name: string, spec: table): MamaCordCommandRoute
---@field user_command fun(name: string, spec: table): MamaCordUserCommandRoute
---@field message_command fun(name: string, spec: table): MamaCordMessageCommandRoute
---@field job fun(id: string, spec: table): MamaCordJobRoute
---@field require fun(path: string): any
---@field include fun(path: string): boolean

---@type MamaCordAPI
bot = bot

---Legacy host API alias kept for older plugins.
---@type table
mamacord = mamacord
