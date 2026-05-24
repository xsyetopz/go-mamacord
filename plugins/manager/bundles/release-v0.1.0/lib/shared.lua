local ui = bot.ui

local M = {}

M.success_color = 0x57F287
M.error_color = 0xED4245

M.max_emoji_file_bytes = 256 * 1024
M.max_sticker_file_bytes = 512 * 1024
M.max_asset_dimension = 320

M.channel_type_guild_text = 0
M.channel_type_guild_voice = 2
M.channel_type_guild_stage_voice = 13
M.channel_type_guild_forum = 15

function M.trim(value)
  if value == nil then
    return ""
  end
  return (tostring(value):gsub("^%s+", ""):gsub("%s+$", ""))
end

function M.mention_user(user_id)
  return "<@" .. tostring(user_id) .. ">"
end

function M.mention_role(role_id)
  return "<@&" .. tostring(role_id) .. ">"
end

function M.mention_channel(channel_id)
  return "<#" .. tostring(channel_id) .. ">"
end

function M.reply_embed(description, color)
  return ui.reply({
    ephemeral = true,
    embeds = {
      {
        description = description,
        color = color,
      }
    }
  })
end

function M.success(description)
  return M.reply_embed(description, M.success_color)
end

function M.error(description)
  return M.reply_embed(description, M.error_color)
end

function M.ensure_guild(ctx, i18n)
  if ctx.guild.id ~= "" then
    return nil
  end
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

function M.attachment(ctx, name)
  local value = M.resolved(ctx, name)
  if type(value) ~= "table" then
    return nil
  end
  if M.trim(value.id) == "" or M.trim(value.filename) == "" or M.trim(value.url) == "" then
    return nil
  end
  return value
end

function M.int_arg(value, fallback)
  local number = tonumber(value)
  if number == nil then
    return fallback
  end
  return math.floor(number)
end

function M.int_range(value, min_value, max_value)
  if value < min_value then
    return min_value
  end
  if value > max_value then
    return max_value
  end
  return value
end

function M.parse_hex_color(raw)
  local text = M.trim(raw)
  if text == "" then
    return nil
  end
  text = text:gsub("^#", "")
  if #text ~= 6 then
    return nil
  end
  if text:match("[^%x]") ~= nil then
    return nil
  end
  return tonumber(text, 16)
end

function M.attachment_extension(file)
  if file == nil then
    return ""
  end
  local filename = M.trim(file.filename)
  local ext = filename:match("%.([^.]+)$")
  if ext == nil then
    return ""
  end
  return string.lower(ext)
end

function M.split_error(err_text)
  local text = M.trim(err_text)
  if text == "" then
    return "", {}
  end

  local parts = {}
  for part in string.gmatch(text, "([^:]+)") do
    table.insert(parts, part)
  end
  local code = parts[1] or text
  table.remove(parts, 1)
  return code, parts
end

return M
