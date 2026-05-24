local option = bot.option
local random = bot.random
local i18n = bot.i18n

local answers = bot.require("lib/eight_ball.lua")
local shared = bot.require("lib/shared.lua")

return bot.command("8ball", {
  description = "Ask the Magic 8 Ball a question.",
  description_id = "cmd.8ball.desc",
  options = {
    option.string("question", {
      description = "What do you want to ask?",
      description_id = "cmd.8ball.opt.question.desc",
      required = true,
      min_length = 3,
      max_length = 255,
    })
  },
  run = function(ctx)
    local question = tostring(ctx.command.args.question or "")
    if not shared.is_open_ended_question(question) then
      return shared.reply_embed({
        ephemeral = true,
        color = shared.colors.error,
        description = i18n.t("fun.8ball.question_error", { Question = question }, nil),
      })
    end

    local answer = tostring(random.choice(answers) or "")
    answer = answer:gsub("%s+$", "")
    answer = answer:gsub("[%.%!%?]+$", "")

    return shared.reply_embed({
      color = shared.colors.fun,
      description = i18n.t("fun.8ball.result", {
        Answer = answer,
        User = shared.mention(ctx.user.id),
      }, nil),
    })
  end
})
