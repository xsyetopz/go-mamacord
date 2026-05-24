local i18n = bot.i18n
local warnings = bot.warnings
local audit = bot.audit
local time = bot.time

local shared = bot.require("lib/shared.lua")

return function(ctx)
  local values = ctx.component.values or {}
  local selected = shared.trim(values[1])
  local flow = shared.parse_unwarn_value(selected)
  if flow == nil then
    return shared.update_text(i18n.t("err.generic", nil, nil))
  end

  if ctx.user.id ~= flow.actor_id then
    return nil
  end
  if ctx.guild.id == "" then
    return shared.update_text(i18n.t("err.generic", nil, nil))
  end
  if time.unix() - flow.issued_at > shared.unwarn_ttl_seconds then
    return shared.update_text(i18n.t("mod.unwarn.expired", nil, nil))
  end

  local list = warnings.list(ctx.guild.id, flow.target_id, shared.unwarn_verify_limit)
  local found = false
  for _, warning in ipairs(list) do
    if warning.id == flow.warning_id then
      found = true
      break
    end
  end
  if not found then
    return shared.update_text(i18n.t("err.generic", nil, nil))
  end

  warnings.delete(flow.warning_id)
  audit.append({
    guild_id = ctx.guild.id,
    actor_id = ctx.user.id,
    action = "warn.delete",
    target_type = "user",
    target_id = flow.target_id,
    created_at = time.unix(),
    meta_json = "{}",
  })

  return shared.update_text(i18n.t("mod.unwarn.success", {
    User = shared.mention(flow.target_id),
  }, nil))
end
