local option = bot.option
local i18n = bot.i18n

local shared = bot.require("lib/shared.lua")
local kawaii = bot.require("lib/kawaii.lua")

local M = {}

function M.build(spec)
  return bot.command(spec.name, {
    description = spec.description,
    description_id = spec.description_id,
    options = {
      option.user("user", {
        description = "Target user",
        description_id = spec.option_desc_id,
        required = true
      })
    },
    run = function(ctx)
      local guild_error = shared.ensure_guild(ctx)
      if guild_error ~= nil then
        return guild_error
      end

      local target_id = tostring(ctx.command.args.user or "")
      local self_error = shared.ensure_other_user(ctx, target_id)
      if self_error ~= nil then
        return self_error
      end

      local gif_url, kawaii_error = kawaii.fetch_gif_or_error(spec.endpoint)
      if kawaii_error ~= nil then
        return kawaii_error
      end

      return shared.reply_embed({
        color = shared.colors.fun,
        description = i18n.t("fun.kawaii.user_mention_only", {
          Emoji = shared.endpoint_emoji(spec.endpoint),
          User = shared.mention(target_id),
        }, nil),
        image_url = gif_url,
        footer = i18n.t("fun.kawaii.footer", nil, nil),
      })
    end
  })
end

return M
