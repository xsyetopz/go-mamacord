return bot.plugin({
  commands = {
    bot.require("commands/warn.lua"),
    bot.require("commands/unwarn.lua"),
  },

  components = {
    unwarn_select = bot.require("components/unwarn_select.lua"),
  }
})
