local option = bot.option
local ui = bot.ui
local i18n = bot.i18n
local shared = bot.require("lib/shared.lua")

local function error_update(message_id)
  return shared.update_embed(shared.error_embed(i18n.t(message_id, nil, nil)))
end

local function user_subcommand()
  return {
    name = "user",
    description = "Look up a user.",
    description_id = "cmd.lookup.sub.user.desc",
    options = {
      option.user("user", {
        description = "User to inspect.",
        description_id = "cmd.lookup.sub.user.opt.user.desc",
      }),
    },
  }
end

local function guild_subcommand()
  return {
    name = "guild",
    description = "Look up this guild.",
    description_id = "cmd.lookup.sub.guild.desc",
    options = {
      option.string("guild_id", {
        description = "Optional: look up another server by ID (owner only).",
        description_id = "cmd.lookup.sub.guild.opt.guild_id.desc",
        required = false,
        min_length = 5,
        max_length = 32,
      }),
    },
  }
end

local function role_subcommand()
  return {
    name = "role",
    description = "Look up a role.",
    description_id = "cmd.lookup.sub.role.desc",
    options = {
      option.role("role", {
        description = "Role to inspect.",
        description_id = "cmd.lookup.sub.role.opt.role.desc",
        required = true,
      }),
    },
  }
end

local function channel_subcommand()
  return {
    name = "channel",
    description = "Look up a channel.",
    description_id = "cmd.lookup.sub.channel.desc",
    options = {
      option.channel("channel", {
        description = "Channel to inspect.",
        description_id = "cmd.lookup.sub.channel.opt.channel.desc",
        required = true,
      }),
    },
  }
end

local function user_lookup(ctx)
  local ok = ui.defer({ ephemeral = true })
  if not ok then
    return ui.reply({
      ephemeral = true,
      content = i18n.t("err.generic", nil, nil),
    })
  end

  local target_id = shared.user_id_from_context(ctx)
  local target = bot.discord.get_user({
    user_id = target_id,
  })
  if target == nil then
    return error_update("info.lookup.user.error")
  end

  local member = nil
  if shared.trim(ctx.guild.id) ~= "" then
    member = bot.discord.get_member({
      guild_id = ctx.guild.id,
      user_id = target_id,
    })
  end

  local color = shared.info_color
  if tonumber(target.accent_color) ~= nil and tonumber(target.accent_color) ~= 0 then
    color = tonumber(target.accent_color)
  end

  local fields = {
    {
      name = i18n.t("info.lookup.user.field.bot", nil, nil),
      value = shared.bool_string(target.bot),
      inline = true,
    },
    {
      name = i18n.t("info.lookup.user.field.system", nil, nil),
      value = shared.bool_string(target.system),
      inline = true,
    },
    {
      name = i18n.t("info.lookup.user.field.locale", nil, nil),
      value = shared.trim(ctx.locale),
      inline = true,
    },
    {
      name = i18n.t("info.lookup.user.field.created", nil, nil),
      value = shared.discord_timestamp(target.created_at),
      inline = true,
    },
  }

  if member ~= nil and tonumber(member.joined_at) ~= nil and tonumber(member.joined_at) > 0 then
    table.insert(fields, {
      name = i18n.t("info.lookup.user.field.joined", nil, nil),
      value = shared.discord_timestamp(member.joined_at),
      inline = true,
    })
  end
  if member ~= nil and type(member.role_ids) == "table" and #member.role_ids > 0 then
    table.insert(fields, {
      name = i18n.t("info.lookup.user.field.roles", nil, nil),
      value = tostring(#member.role_ids),
      inline = true,
    })
  end

  local embed = {
    title = shared.trim(target.display_name),
    color = color,
    fields = fields,
    footer = "🆔" .. target_id,
  }
  if shared.trim(target.avatar_url) ~= "" then
    embed.thumbnail_url = target.avatar_url
  end
  if shared.trim(target.banner_url) ~= "" then
    embed.image_url = target.banner_url
  end

  return shared.update_embed(embed)
end

local function guild_lookup(ctx)
  local target_guild_id = shared.trim(ctx.command.args.guild_id)
  if target_guild_id == "" then
    if shared.trim(ctx.guild.id) == "" then
      return shared.not_in_guild(i18n)
    end
    target_guild_id = shared.trim(ctx.guild.id)
  end

  local ok = ui.defer({ ephemeral = true })
  if not ok then
    return ui.reply({
      ephemeral = true,
      content = i18n.t("err.generic", nil, nil),
    })
  end

  if target_guild_id:match("^%d+$") == nil then
    return shared.update_embed(shared.error_embed(i18n.t("info.lookup.guild.invalid_guild_id", { GuildID = target_guild_id }, nil)))
  end

  if target_guild_id ~= shared.trim(ctx.guild.id) and ctx.is_owner ~= true then
    return shared.update_embed(shared.error_embed(i18n.t("info.lookup.guild.owner_only", nil, nil)))
  end

  local guild, guild_err = bot.discord.get_guild({ guild_id = target_guild_id })
  if guild == nil then
    return shared.update_embed(shared.error_embed(i18n.t("info.lookup.guild.not_accessible", { GuildID = target_guild_id, Error = tostring(guild_err or "") }, nil)))
  end

  local owner = bot.discord.get_user({ user_id = guild.owner_id })
  local embed = {
    title = shared.trim(guild.name),
    description = shared.trim(guild.description),
    color = shared.info_color,
    fields = {
      {
        name = i18n.t("info.lookup.guild.field.roles", nil, nil),
        value = tostring(guild.roles_count),
        inline = true,
      },
      {
        name = i18n.t("info.lookup.guild.field.emojis", nil, nil),
        value = tostring(guild.emojis_count),
        inline = true,
      },
      {
        name = i18n.t("info.lookup.guild.field.stickers", nil, nil),
        value = tostring(guild.stickers_count),
        inline = true,
      },
      {
        name = i18n.t("info.lookup.guild.field.members", nil, nil),
        value = tostring(guild.member_count),
        inline = true,
      },
      {
        name = i18n.t("info.lookup.guild.field.channels", nil, nil),
        value = tostring(guild.channels_count),
        inline = true,
      },
      {
        name = i18n.t("info.lookup.guild.field.created", nil, nil),
        value = shared.discord_timestamp(guild.created_at),
        inline = true,
      },
    },
    footer = "🆔" .. shared.trim(target_guild_id),
  }

  if owner ~= nil then
    embed.author = { name = shared.trim(owner.username) }
    if shared.trim(owner.avatar_url) ~= "" then
      embed.author.icon_url = owner.avatar_url
    end
  end
  if shared.trim(guild.icon_url) ~= "" then
    embed.thumbnail_url = guild.icon_url
  end
  if shared.trim(guild.banner_url) ~= "" then
    embed.image_url = guild.banner_url
  end

  return shared.update_embed(embed)
end

local function role_lookup(ctx)
  if shared.trim(ctx.guild.id) == "" then
    return shared.not_in_guild(i18n)
  end

  local role = shared.resolved(ctx, "role")
  if shared.trim(role.id) == "" then
    return ui.reply({
      ephemeral = true,
      content = i18n.t("err.generic", nil, nil),
    })
  end

  local embed = {
    title = shared.trim(role.name),
    color = tonumber(role.color) or shared.info_color,
    fields = {
      {
        name = i18n.t("info.lookup.role.field.mention", nil, nil),
        value = shared.trim(role.mention),
        inline = true,
      },
      {
        name = i18n.t("info.lookup.role.field.position", nil, nil),
        value = tostring(role.position or 0),
        inline = true,
      },
      {
        name = i18n.t("info.lookup.role.field.hoist", nil, nil),
        value = shared.bool_string(role.hoist),
        inline = true,
      },
      {
        name = i18n.t("info.lookup.role.field.mentionable", nil, nil),
        value = shared.bool_string(role.mentionable),
        inline = true,
      },
      {
        name = i18n.t("info.lookup.role.field.managed", nil, nil),
        value = shared.bool_string(role.managed),
        inline = true,
      },
      {
        name = i18n.t("info.lookup.role.field.permissions", nil, nil),
        value = "`" .. tostring(role.permissions or 0) .. "`",
        inline = true,
      },
      {
        name = i18n.t("info.lookup.role.field.created", nil, nil),
        value = shared.discord_timestamp(role.created_at),
        inline = true,
      },
    },
    footer = "🆔" .. tostring(role.id),
  }

  return shared.reply_embed(embed)
end

local function channel_lookup(ctx)
  if shared.trim(ctx.guild.id) == "" then
    return shared.not_in_guild(i18n)
  end

  local channel = shared.resolved(ctx, "channel")
  if shared.trim(channel.id) == "" then
    return ui.reply({
      ephemeral = true,
      content = i18n.t("err.generic", nil, nil),
    })
  end

  local fields = {
    {
      name = i18n.t("info.lookup.channel.field.mention", nil, nil),
      value = shared.trim(channel.mention),
      inline = true,
    },
    {
      name = i18n.t("info.lookup.channel.field.type", nil, nil),
      value = shared.trim(channel.type),
      inline = true,
    },
    {
      name = i18n.t("info.lookup.channel.field.permissions", nil, nil),
      value = "`" .. tostring(channel.permissions or 0) .. "`",
      inline = true,
    },
    {
      name = i18n.t("info.lookup.channel.field.created", nil, nil),
      value = shared.discord_timestamp(channel.created_at),
      inline = true,
    },
  }
  if shared.trim(channel.parent_id) ~= "" then
    table.insert(fields, {
      name = i18n.t("info.lookup.channel.field.parent", nil, nil),
      value = "<#" .. tostring(channel.parent_id) .. ">",
      inline = true,
    })
  end

  return shared.reply_embed({
    title = shared.trim(channel.name),
    color = shared.info_color,
    fields = fields,
    footer = "🆔" .. tostring(channel.id),
  })
end

return bot.command("lookup", {
  description = "Look up Discord objects.",
  description_id = "cmd.lookup.desc",
  ephemeral = true,
  subcommands = {
    user_subcommand(),
    guild_subcommand(),
    role_subcommand(),
    channel_subcommand(),
  },
  run = function(ctx)
    local subcommand = shared.trim(ctx.command.subcommand)
    if subcommand == "user" then
      return user_lookup(ctx)
    end
    if subcommand == "guild" then
      return guild_lookup(ctx)
    end
    if subcommand == "role" then
      return role_lookup(ctx)
    end
    if subcommand == "channel" then
      return channel_lookup(ctx)
    end

    return ui.reply({
      ephemeral = true,
      content = i18n.t("err.generic", nil, nil),
    })
  end
})
