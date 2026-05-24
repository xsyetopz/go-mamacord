local option = bot.option
local i18n = bot.i18n
local usersettings = bot.usersettings

local shared = bot.require("lib/shared.lua")

return bot.command("timezone", {
  description = "Manage your timezone.",
  description_id = "cmd.timezone.desc",
  ephemeral = true,
  subcommands = {
    {
      name = "set",
      description = "Save your IANA timezone.",
      description_id = "cmd.timezone.sub.set.desc",
      options = {
        option.string("iana", {
          description = "Timezone like Europe/Tallinn.",
          description_id = "cmd.timezone.opt.iana.desc",
          required = true,
          min_length = 1,
          max_length = 64,
        }),
      },
    },
    {
      name = "show",
      description = "Show your saved timezone.",
      description_id = "cmd.timezone.sub.show.desc",
    },
    {
      name = "clear",
      description = "Clear your saved timezone.",
      description_id = "cmd.timezone.sub.clear.desc",
    },
  },
  run = function(ctx)
    local subcommand = shared.trim(ctx.command.subcommand)

    if subcommand == "set" then
      local timezone_raw = shared.trim(ctx.command.args.iana)
      local timezone_name = usersettings.normalize_timezone(timezone_raw)
      if timezone_name == nil then
        return shared.reply_text(i18n.t("wellness.timezone.invalid", {
          Timezone = timezone_raw,
        }, nil), true)
      end

      usersettings.set_timezone(ctx.user.id, timezone_name)
      return shared.reply_text(i18n.t("wellness.timezone.set", {
        Timezone = timezone_name,
      }, nil), true)
    end

    if subcommand == "show" then
      local settings, ok = usersettings.get(ctx.user.id)
      local timezone_name = ""
      if ok and settings ~= nil then
        timezone_name = shared.trim(settings.timezone)
      end
      if timezone_name == "" then
        return shared.reply_text(i18n.t("wellness.timezone.unset", nil, nil), true)
      end
      return shared.reply_text(i18n.t("wellness.timezone.show", {
        Timezone = timezone_name,
      }, nil), true)
    end

    if subcommand == "clear" then
      usersettings.clear_timezone(ctx.user.id)
      return shared.reply_text(i18n.t("wellness.timezone.cleared", nil, nil), true)
    end

    return shared.reply_text(i18n.t("err.generic", nil, nil), true)
  end
})
