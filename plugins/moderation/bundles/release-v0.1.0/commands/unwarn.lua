local option = bot.option
local i18n = bot.i18n
local warnings = bot.warnings
local ui = bot.ui
local time = bot.time

local shared = bot.require("lib/shared.lua")

return bot.command("unwarn", {
  description = "Remove one warning from a member.",
  description_id = "cmd.unwarn.desc",
  ephemeral = true,
  default_member_permissions = { "moderate_members" },
  options = {
    option.user("user", {
      description = "User whose warning should be removed.",
      description_id = "cmd.unwarn.opt.user.desc",
      required = true,
    }),
  },
  run = function(ctx)
    local guild_error = shared.ensure_guild(ctx, i18n.t("err.not_in_guild", nil, nil))
    if guild_error ~= nil then
      return guild_error
    end

    local target_id = shared.trim(ctx.command.args.user)
    local resolved = shared.resolved_user(ctx, "user")

    if target_id == ctx.user.id then
      return shared.reply_text(i18n.t("mod.unwarn.self", nil, nil), true)
    end
    if resolved.bot or resolved.system then
      return shared.reply_text(i18n.t("mod.warn.bot", nil, nil), true)
    end

    local list = warnings.list(ctx.guild.id, target_id, shared.unwarn_list_limit)
    if #list == 0 then
      return shared.reply_text(i18n.t("mod.unwarn.none", {
        User = shared.mention(target_id),
      }, nil), true)
    end

    local issued_at = time.unix()
    local options = {}
    for _, warning in ipairs(list) do
      table.insert(options, ui.string_option(shared.warning_label(warning), shared.unwarn_value(
        warning.id,
        ctx.user.id,
        target_id,
        issued_at
      )))
    end

    return ui.reply({
      ephemeral = true,
      content = i18n.t("mod.unwarn.prompt", nil, nil),
      components = {
        {
          ui.string_select("unwarn_select", {
            placeholder = i18n.t("mod.unwarn.placeholder", nil, nil),
            min_values = 1,
            max_values = 1,
            options = options,
          })
        }
      }
    })
  end
})
