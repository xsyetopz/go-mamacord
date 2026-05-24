local ui = bot.ui

local M = {}

M.info_color = 0x5865F2
M.error_color = 0xED4245

function M.trim(value)
  if value == nil then
    return ""
  end
  return (tostring(value):gsub("^%s+", ""):gsub("%s+$", ""))
end

function M.bool_string(value)
  if value then
    return "true"
  end
  return "false"
end

function M.discord_timestamp(unix)
  local number = tonumber(unix)
  if number == nil or number <= 0 then
    return "UNKNOWN"
  end
  return "<t:" .. tostring(math.floor(number)) .. ":F>"
end

function M.https_url(value)
  local text = string.lower(M.trim(value))
  if text:sub(1, 8) ~= "https://" then
    return nil
  end
  return M.trim(value)
end

function M.reply_embed(embed)
  return ui.reply({
    ephemeral = true,
    embeds = { embed },
  })
end

function M.update_embed(embed)
  return ui.update({
    embeds = { embed },
  })
end

function M.error_embed(description)
  return {
    description = M.trim(description),
    color = M.error_color,
  }
end

function M.not_in_guild(i18n)
  return ui.reply({
    ephemeral = true,
    content = i18n.t("err.not_in_guild", nil, nil),
  })
end

function M.resolved(ctx, name)
  local command = ctx.command or {}
  local resolved = command.resolved or {}
  local value = resolved[name]
  if type(value) == "table" then
    return value
  end
  return {}
end

function M.user_id_from_context(ctx)
  local raw = M.trim(ctx.command.args.user)
  if raw ~= "" then
    return raw
  end
  return M.trim(ctx.user.id)
end

return M
