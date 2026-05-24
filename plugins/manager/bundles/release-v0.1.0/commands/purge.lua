local option = bot.option
local i18n = bot.i18n
local shared = bot.require("lib/shared.lua")

local PURGE_DEFAULT_COUNT = 2
local PURGE_MIN_COUNT = 1
local PURGE_MAX_COUNT = 100

local function count_option(min_value)
  return option.int("count", {
    description = "How many messages to delete.",
    description_id = "cmd.purge.opt.count.desc",
    min_value = min_value,
    max_value = PURGE_MAX_COUNT,
  })
end

local function message_option()
  return option.string("message", {
    description = "Message ID or link to anchor around.",
    description_id = "cmd.purge.opt.message.desc",
    required = true,
    max_length = 255,
  })
end

local function subcommand(name, description_id, min_count, needs_message)
  local options = {
    count_option(min_count),
  }
  if needs_message then
    table.insert(options, 1, message_option())
  end
  return {
    name = name,
    description = "Purge messages.",
    description_id = description_id,
    options = options,
  }
end

return bot.command("purge", {
  description = "Delete messages in bulk.",
  description_id = "cmd.purge.desc",
  ephemeral = true,
  default_member_permissions = { "manage_messages" },
  subcommands = {
    subcommand("all", "cmd.purge.sub.all.desc", 2, false),
    subcommand("before", "cmd.purge.sub.before.desc", 1, true),
    subcommand("after", "cmd.purge.sub.after.desc", 1, true),
    subcommand("around", "cmd.purge.sub.around.desc", 2, true),
  },
  run = function(ctx)
    local guild_error = shared.ensure_guild(ctx, i18n)
    if guild_error ~= nil then
      return guild_error
    end

    local mode = shared.trim(ctx.command.subcommand)
    local count = shared.int_range(shared.int_arg(ctx.command.args.count, PURGE_DEFAULT_COUNT), PURGE_MIN_COUNT, PURGE_MAX_COUNT)
    local anchor = shared.trim(ctx.command.args.message)
    if mode ~= "all" and anchor == "" then
      return shared.error(i18n.t("mgr.purge.invalid_message", nil, nil))
    end

    local result, err = bot.discord.purge_messages({
      channel_id = ctx.channel.id,
      mode = mode,
      anchor_message_id = anchor,
      count = count,
    })
    if result == nil then
      local code = shared.split_error(err)
      if code == "invalid_message" then
        return shared.error(i18n.t("mgr.purge.invalid_message", nil, nil))
      end
      return shared.error(i18n.t("mgr.purge.error", nil, nil))
    end

    return shared.success(i18n.t("mgr.purge.success", {
      Count = result.deleted_count,
    }, nil))
  end
})
