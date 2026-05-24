local random = bot.random
local i18n = bot.i18n

local shared = bot.require("lib/shared.lua")

return bot.command("flip", {
  description = "Flip a coin.",
  description_id = "cmd.flip.desc",
  run = function(ctx)
    local result = "heads"
    if random.int(0, 1) == 0 then
      result = "tails"
    end

    return shared.reply_embed({
      color = shared.colors.fun,
      description = i18n.t("fun.flip.result", {
        User = shared.mention(ctx.user.id),
        Result = result,
      }, nil),
    })
  end
})
