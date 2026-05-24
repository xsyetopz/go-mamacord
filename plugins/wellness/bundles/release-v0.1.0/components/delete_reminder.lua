local i18n = bot.i18n

local shared = bot.require("lib/shared.lua")

return function(ctx)
  local values = ctx.component.values or {}
  local reminder_id = shared.trim(values[1])
  if reminder_id == "" then
    return shared.update_text(i18n.t("err.generic", nil, nil))
  end

  local deleted = bot.reminders.delete(ctx.user.id, reminder_id)
  if not deleted then
    return shared.update_text(i18n.t("wellness.remind.delete.not_found", nil, nil))
  end

  return shared.update_text(i18n.t("wellness.remind.delete.success", nil, nil))
end
