local kawaii_command = bot.require("lib/kawaii_command.lua")

return kawaii_command.build({
  name = "hug",
  description = "Give someone a warm hug.",
  description_id = "cmd.hug.desc",
  option_desc_id = "cmd.hug.opt.user.desc",
  endpoint = "hug",
})
