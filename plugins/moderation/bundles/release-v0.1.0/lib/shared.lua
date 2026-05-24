local ui = bot.ui

local M = {}

M.warn_max = 3
M.warn_timeout_seconds = 10 * 60
M.unwarn_verify_limit = 100
M.unwarn_list_limit = 25
M.unwarn_ttl_seconds = 120
M.config_key = "__guild_config"

function M.trim(value)
  if value == nil then
    return ""
  end
  return (tostring(value):gsub("^%s+", ""):gsub("%s+$", ""))
end

function M.mention(user_id)
  return "<@" .. tostring(user_id) .. ">"
end

function M.reply_text(message, ephemeral)
  return ui.reply({
    ephemeral = ephemeral,
    content = message,
  })
end

function M.update_text(message)
  return ui.update({
    content = message,
    components = {},
  })
end

function M.ensure_guild(ctx, message)
  if ctx.guild.id ~= "" then
    return nil
  end
  return M.reply_text(message, true)
end

function M.guild_config(ctx)
  local config = {
    enabled = true,
    commands = {
      warn = true,
      unwarn = true,
    },
    warning_limit = M.warn_max,
    timeout_threshold = M.warn_max,
    timeout_minutes = math.floor(M.warn_timeout_seconds / 60),
  }

  if ctx == nil or ctx.guild == nil or ctx.guild.id == "" or ctx.store == nil then
    return config
  end

  local read_ok, stored, ok = pcall(function()
    return ctx.store.get(M.config_key)
  end)
  if not read_ok or not ok or type(stored) ~= "table" then
    return config
  end

  if type(stored.enabled) == "boolean" then
    config.enabled = stored.enabled
  end
  if type(stored.commands) == "table" then
    if type(stored.commands.warn) == "boolean" then
      config.commands.warn = stored.commands.warn
    end
    if type(stored.commands.unwarn) == "boolean" then
      config.commands.unwarn = stored.commands.unwarn
    end
  end
  if tonumber(stored.warning_limit) ~= nil and tonumber(stored.warning_limit) > 0 then
    config.warning_limit = math.floor(tonumber(stored.warning_limit))
  end
  if tonumber(stored.timeout_threshold) ~= nil and tonumber(stored.timeout_threshold) > 0 then
    config.timeout_threshold = math.floor(tonumber(stored.timeout_threshold))
  end
  if tonumber(stored.timeout_minutes) ~= nil and tonumber(stored.timeout_minutes) > 0 then
    config.timeout_minutes = math.floor(tonumber(stored.timeout_minutes))
  end
  return config
end

function M.warning_label(warning)
  local label = "mod " .. tostring(warning.moderator_id) .. " - " .. tostring(warning.created_at)
  if #label > 100 then
    label = label:sub(1, 100)
  end
  return label
end

function M.timeout_member(guild_id, user_id, until_unix)
  return bot.discord.timeout_member({
    guild_id = guild_id,
    user_id = user_id,
    until_unix = until_unix,
  })
end

function M.send_dm(user_id, message)
  return bot.discord.send_dm({
    user_id = user_id,
    message = message,
  })
end

function M.unwarn_value(warning_id, actor_id, target_id, issued_at)
  return table.concat({
    tostring(warning_id),
    tostring(actor_id),
    tostring(target_id),
    tostring(issued_at),
  }, "|")
end

function M.parse_unwarn_value(raw)
  local text = M.trim(raw)
  if text == "" then
    return nil
  end

  local parts = {}
  for part in string.gmatch(text, "([^|]+)") do
    table.insert(parts, part)
  end
  if #parts ~= 4 then
    return nil
  end

  local issued_at = tonumber(parts[4])
  if issued_at == nil then
    return nil
  end

  return {
    warning_id = parts[1],
    actor_id = parts[2],
    target_id = parts[3],
    issued_at = issued_at,
  }
end

function M.resolved_user(ctx, name)
  local command = ctx.command or {}
  local resolved = command.resolved or {}
  local value = resolved[name]
  if type(value) == "table" then
    return value
  end
  return {}
end

return M
