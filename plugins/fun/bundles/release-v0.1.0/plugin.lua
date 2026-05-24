return bot.plugin({
  commands = {
    bot.require("commands/flip.lua"),
    bot.require("commands/roll.lua"),
    bot.require("commands/8ball.lua"),
    bot.require("commands/hug.lua"),
    bot.require("commands/pat.lua"),
    bot.require("commands/poke.lua"),
    bot.require("commands/shrug.lua"),
  }
})
