return bot.plugin({
  commands = {
    bot.require("commands/about.lua"),
    bot.require("commands/lookup.lua"),
  }
})
