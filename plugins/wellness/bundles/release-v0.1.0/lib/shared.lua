local ui = bot.ui

local M = {}

M.reminder_kinds = {
  "hydrate",
  "stretch",
  "breathe",
  "meds",
  "sleep",
  "checkin",
}

M.reminder_kind_choices = {
  { name = "hydrate", value = "hydrate" },
  { name = "stretch", value = "stretch" },
  { name = "breathe", value = "breathe" },
  { name = "meds",    value = "meds" },
  { name = "sleep",   value = "sleep" },
  { name = "checkin", value = "checkin" },
}

M.delivery_choices = {
  { name = "dm",      value = "dm" },
  { name = "channel", value = "channel" },
}
M.config_key = "__guild_config"

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

function M.trim(value)
  if value == nil then
    return ""
  end
  return (tostring(value):gsub("^%s+", ""):gsub("%s+$", ""))
end

function M.timestamp(unix_seconds)
  return "<t:" .. tostring(unix_seconds) .. ":f>"
end

function M.ensure_guild(ctx, message)
  if ctx.guild.id ~= "" then
    return nil
  end
  return M.reply_text(message, true)
end

function M.reminder_option_label(reminder)
  local kind = M.trim(reminder.kind)
  if kind == "" then
    kind = reminder.id
  end
  local label = kind .. " • " .. M.timestamp(reminder.next_run_at)
  if #label > 90 then
    label = label:sub(1, 90)
  end
  return label
end

function M.guild_config(ctx)
  local config = {
    enabled = true,
    commands = {
      timezone = true,
      checkin = true,
      remind = true,
    },
    allow_channel_reminders = true,
    default_reminder_channel_id = "",
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
    if type(stored.commands.timezone) == "boolean" then
      config.commands.timezone = stored.commands.timezone
    end
    if type(stored.commands.checkin) == "boolean" then
      config.commands.checkin = stored.commands.checkin
    end
    if type(stored.commands.remind) == "boolean" then
      config.commands.remind = stored.commands.remind
    end
  end
  if type(stored.allow_channel_reminders) == "boolean" then
    config.allow_channel_reminders = stored.allow_channel_reminders
  end
  if stored.default_reminder_channel_id ~= nil then
    config.default_reminder_channel_id = M.trim(stored.default_reminder_channel_id)
  end
  return config
end

return M
