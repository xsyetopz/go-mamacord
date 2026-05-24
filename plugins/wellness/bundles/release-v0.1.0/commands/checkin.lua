local option = bot.option
local i18n = bot.i18n
local checkins = bot.checkins

local shared = bot.require("lib/shared.lua")

local HISTORY_LIMIT = 10

return bot.command("checkin", {
  description = "Save how you're feeling today.",
  description_id = "cmd.checkin.desc",
  ephemeral = true,
  options = {
    option.int("mood", {
      description = "Mood from 1 to 5.",
      description_id = "cmd.checkin.opt.mood.desc",
      min_value = 1,
      max_value = 5,
    }),
    option.bool("history", {
      description = "Show recent check-ins instead.",
      description_id = "cmd.checkin.opt.history.desc",
    }),
  },
  run = function(ctx)
    local mood = tonumber(ctx.command.args.mood)
    local wants_history = ctx.command.args.history == true

    if wants_history and mood == nil then
      local entries = checkins.list(ctx.user.id, HISTORY_LIMIT)
      if #entries == 0 then
        return shared.reply_text(i18n.t("wellness.checkin.history.empty", nil, nil), true)
      end

      local lines = {}
      for _, entry in ipairs(entries) do
        table.insert(lines, "- " .. shared.timestamp(entry.created_at) .. ": " .. tostring(entry.mood) .. "/5")
      end

      return shared.reply_text(i18n.t("wellness.checkin.history", {
        Lines = table.concat(lines, "\n"),
      }, nil), true)
    end

    if mood == nil then
      return shared.reply_text(i18n.t("wellness.checkin.prompt", nil, nil), true)
    end
    if mood < 1 or mood > 5 then
      return shared.reply_text(i18n.t("wellness.checkin.invalid_mood", nil, nil), true)
    end

    checkins.create({
      user_id = ctx.user.id,
      mood = mood,
    })
    return shared.reply_text(i18n.t("wellness.checkin.saved", {
      Mood = mood,
    }, nil), true)
  end
})
