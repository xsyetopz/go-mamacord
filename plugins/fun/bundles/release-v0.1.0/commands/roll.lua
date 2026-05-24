local option = bot.option
local random = bot.random
local i18n = bot.i18n

local shared = bot.require("lib/shared.lua")

return bot.command("roll", {
  description = "Roll some dice.",
  description_id = "cmd.roll.desc",
  options = {
    option.int("number", {
      description = "How many dice?",
      description_id = "cmd.roll.opt.number.desc",
      required = true,
      min_value = 1,
      max_value = 99,
    }),
    option.int("sides", {
      description = "How many sides per die?",
      description_id = "cmd.roll.opt.sides.desc",
      required = true,
      min_value = 4,
      max_value = 20,
    }),
    option.int("modifier", {
      description = "Any modifier to add?",
      description_id = "cmd.roll.opt.modifier.desc",
      min_value = -99,
      max_value = 99,
    }),
  },
  run = function(ctx)
    local number = tonumber(ctx.command.args.number or 0) or 0
    local sides = tonumber(ctx.command.args.sides or 0) or 0
    local modifier = tonumber(ctx.command.args.modifier or 0) or 0

    if not shared.is_allowed_dice_sides(sides) then
      return shared.reply_embed({
        ephemeral = true,
        color = shared.colors.warn,
        description = i18n.t("fun.roll.invalid_sides", { Sides = sides }, nil),
      })
    end

    local rolls = {}
    local sum = 0
    for _ = 1, number do
      local value = random.int(1, sides)
      table.insert(rolls, value)
      sum = sum + value
    end

    local verbose = table.concat(rolls, ", ")
    if modifier > 0 then
      verbose = verbose .. " + " .. tostring(modifier)
    elseif modifier < 0 then
      verbose = verbose .. " - " .. tostring(-modifier)
    end

    return shared.reply_embed({
      color = shared.colors.fun,
      description = i18n.t("fun.roll.result", {
        User = shared.mention(ctx.user.id),
        Notation = shared.roll_notation(number, sides, modifier),
        Total = sum + modifier,
      }, nil),
      footer = verbose,
    })
  end
})
