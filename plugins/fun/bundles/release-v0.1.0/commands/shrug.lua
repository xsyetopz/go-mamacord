local option = bot.option
local ui = bot.ui
local i18n = bot.i18n

local shared = bot.require("lib/shared.lua")
local kawaii = bot.require("lib/kawaii.lua")

return bot.command("shrug", {
  description = "Send a shrug.",
  description_id = "cmd.shrug.desc",
  options = {
    option.string("message", {
      description = "Anything to add?",
      description_id = "cmd.shrug.opt.message.desc",
      max_length = 2000,
    })
  },
  run = function(ctx)
    local guild_error = shared.ensure_guild(ctx)
    if guild_error ~= nil then
      return guild_error
    end

    ---@type string|nil
    local content = nil
    local raw_message = ctx.command.args.message
    if raw_message ~= nil then
      local text = tostring(raw_message)
      if text ~= "" then
        content = text
      end
    end

    local gif_url, kawaii_error = kawaii.fetch_gif_or_error("shrug")
    if kawaii_error ~= nil then
      return kawaii_error
    end

    return ui.reply({
      content = content,
      embeds = {
        {
          color = shared.colors.fun,
          image_url = gif_url,
          footer = i18n.t("fun.kawaii.footer", nil, nil),
        }
      }
    })
  end
})
