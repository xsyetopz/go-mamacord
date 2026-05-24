local i18n = bot.i18n
local shared = bot.require("lib/shared.lua")

local function author_table(self_user, build)
  if self_user == nil then
    return nil
  end

  local name = shared.trim(self_user.username)
  local version = shared.trim(build.version)
  if name ~= "" and version ~= "" then
    name = name .. " " .. version
  end

  local author = { name = name }
  if shared.trim(self_user.avatar_url) ~= "" then
    author.icon_url = self_user.avatar_url
  end
  if shared.trim(author.name) == "" and shared.trim(author.icon_url) == "" then
    return nil
  end
  return author
end

return bot.command("about", {
  description = "Show bot details.",
  description_id = "cmd.about.desc",
  ephemeral = true,
  run = function(ctx)
    local build = bot.runtime.build_info()
    local self_user = bot.discord.self_user()
    local caller = bot.discord.get_user()

    local embed = {
      title = i18n.t("info.about.title", {
        Version = shared.trim(build.version),
      }, nil),
      description = shared.trim(build.description),
      color = shared.info_color,
      author = author_table(self_user, build),
    }

    local repo = shared.https_url(build.repository)
    if repo ~= nil then
      embed.url = repo
    end

    local mascot = shared.https_url(build.mascot_image_url)
    if mascot ~= nil then
      embed.image_url = mascot
    end

    if caller ~= nil then
      embed.footer = {
        text = shared.trim(caller.username),
        icon_url = shared.https_url(caller.avatar_url),
      }
    end

    return shared.reply_embed(embed)
  end
})
