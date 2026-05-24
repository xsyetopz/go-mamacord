return bot.plugin({
  commands = {
    bot.require("commands/timezone.lua"),
    bot.require("commands/checkin.lua"),
    bot.require("commands/remind.lua"),
  },

  components = {
    delete_reminder = bot.require("components/delete_reminder.lua"),
  }
})
