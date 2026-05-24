local kawaii_command = bot.require("lib/kawaii_command.lua")

return kawaii_command.build({
  name = "poke",
  description = "Give someone a tiny poke.",
  description_id = "cmd.poke.desc",
  option_desc_id = "cmd.poke.opt.user.desc",
  endpoint = "poke",
})
