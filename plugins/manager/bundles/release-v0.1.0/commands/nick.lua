local option = bot.option
local i18n = bot.i18n
local ui = bot.ui
local shared = bot.require("lib/shared.lua")

return bot.command("nick", {
  description = "Change a member nickname.",
  description_id = "cmd.nick.desc",
  ephemeral = true,
  default_member_permissions = { "manage_nicknames" },
  options = {
    option.user("user", {
      description = "User to rename.",
      description_id = "cmd.nick.opt.user.desc",
      required = true,
    }),
    option.string("nickname", {
      description = "Nickname to set, or leave empty to reset.",
      description_id = "cmd.nick.opt.nickname.desc",
      min_length = 1,
      max_length = 32,
    }),
  },
  run = function(ctx)
    local guild_error = shared.ensure_guild(ctx, i18n)
    if guild_error ~= nil then
      return guild_error
    end

    local target_id = shared.trim(ctx.command.args.user)
    local nickname = shared.trim(ctx.command.args.nickname)
    local target = shared.resolved(ctx, "user")

    if target_id == ctx.user.id then
      return ui.reply({
        ephemeral = true,
        content = i18n.t("mgr.nick.self_error", nil, nil),
      })
    end
    if target.bot or target.system then
      return ui.reply({
        ephemeral = true,
        content = i18n.t("mod.warn.bot", nil, nil),
      })
    end

    local ok, err = bot.discord.set_nickname({
      guild_id = ctx.guild.id,
      user_id = target_id,
      nickname = nickname,
    })
    if not ok then
      return shared.error(i18n.t("mgr.nick.error", {
        User = shared.mention_user(target_id),
        Nickname = nickname,
      }, nil))
    end

    if nickname == "" then
      return shared.success(i18n.t("mgr.nick.reset", {
        User = shared.mention_user(target_id),
      }, nil))
    end

    return shared.success(i18n.t("mgr.nick.set", {
      User = shared.mention_user(target_id),
      Nickname = nickname,
    }, nil))
  end
})
