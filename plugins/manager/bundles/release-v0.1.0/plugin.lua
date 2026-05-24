return bot.plugin({
  commands = {
    bot.require("commands/slowmode.lua"),
    bot.require("commands/nick.lua"),
    bot.require("commands/roles.lua"),
    bot.require("commands/purge.lua"),
    bot.require("commands/emojis.lua"),
    bot.require("commands/stickers.lua"),
  }
})
