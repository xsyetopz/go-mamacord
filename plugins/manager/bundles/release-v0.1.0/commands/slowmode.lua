local option = bot.option
local i18n = bot.i18n
local shared = bot.require("lib/shared.lua")

return bot.command("slowmode", {
  description = "Set per-user channel slowmode.",
  description_id = "cmd.slowmode.desc",
  ephemeral = true,
  default_member_permissions = { "manage_channels" },
  options = {
    option.channel("channel", {
      description = "Channel to update.",
      description_id = "cmd.slowmode.opt.channel.desc",
      channel_types = {
        shared.channel_type_guild_text,
        shared.channel_type_guild_voice,
        shared.channel_type_guild_stage_voice,
        shared.channel_type_guild_forum,
      },
    }),
    option.int("seconds", {
      description = "Slowmode duration in seconds.",
      description_id = "cmd.slowmode.opt.seconds.desc",
      min_value = 1,
      max_value = 21600,
    }),
  },
  run = function(ctx)
    local guild_error = shared.ensure_guild(ctx, i18n)
    if guild_error ~= nil then
      return guild_error
    end

    local channel_id = shared.trim(ctx.command.args.channel)
    if channel_id == "" then
      channel_id = ctx.channel.id
    end

    local seconds = shared.int_arg(ctx.command.args.seconds, 0)
    local ok, err = bot.discord.set_slowmode({
      channel_id = channel_id,
      seconds = seconds,
    })
    if not ok then
      return shared.error(i18n.t("mgr.slowmode.error", nil, nil))
    end

    if seconds == 0 then
      return shared.success(i18n.t("mgr.slowmode.removed", {
        Channel = shared.mention_channel(channel_id),
      }, nil))
    end

    return shared.success(i18n.t("mgr.slowmode.set", {
      Channel = shared.mention_channel(channel_id),
      Seconds = seconds,
    }, nil))
  end
})
