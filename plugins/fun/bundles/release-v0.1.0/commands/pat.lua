local kawaii_command = bot.require("lib/kawaii_command.lua")

return kawaii_command.build({
  name = "pat",
  description = "Give someone a gentle head-pat.",
  description_id = "cmd.pat.desc",
  option_desc_id = "cmd.pat.opt.user.desc",
  endpoint = "pat",
})
