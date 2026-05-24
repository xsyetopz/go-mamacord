local ui = bot.ui
local i18n = bot.i18n

local M = {}

M.colors = {
  fun = 0x5865F2,
  warn = 0xFEE75C,
  error = 0xED4245,
}

function M.mention(user_id)
  return "<@" .. tostring(user_id) .. ">"
end

function M.embed(spec)
  return {
    description = spec.description,
    color = spec.color,
    image_url = spec.image_url,
    footer = spec.footer,
  }
end

function M.reply_embed(spec)
  return ui.reply({
    ephemeral = spec.ephemeral,
    content = spec.content,
    embeds = { M.embed(spec) },
  })
end

function M.ensure_guild(ctx)
  if ctx.guild.id ~= "" then
    return nil
  end
  return ui.reply({
    ephemeral = true,
    content = "This command must be used in a server."
  })
end

function M.ensure_other_user(ctx, target_id)
  if tostring(target_id) ~= tostring(ctx.user.id) then
    return nil
  end
  return ui.reply({
    ephemeral = true,
    content = i18n.t("fun.kawaii.self_error", nil, nil),
  })
end

function M.is_allowed_dice_sides(sides)
  return sides == 4 or sides == 6 or sides == 8 or sides == 10 or sides == 12 or sides == 20
end

function M.roll_notation(number, sides, modifier)
  local base = tostring(number) .. "d" .. tostring(sides)
  if modifier > 0 then
    return base .. "+" .. tostring(modifier)
  end
  if modifier < 0 then
    return base .. tostring(modifier)
  end
  return base
end

function M.is_open_ended_question(question)
  if question == nil then
    return false
  end
  if #question < 3 then
    return false
  end
  local last = question:sub(-1)
  return last == "?" or last == "." or last == "!"
end

function M.endpoint_emoji(endpoint)
  if endpoint == "hug" then
    return "🤗"
  end
  if endpoint == "pat" then
    return "🫳"
  end
  if endpoint == "poke" then
    return "👉"
  end
  if endpoint == "shrug" then
    return "🤷"
  end
  return "✨"
end

return M
