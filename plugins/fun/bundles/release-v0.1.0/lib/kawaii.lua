local shared = bot.require("lib/shared.lua")
local i18n = bot.i18n

local M = {}

local allowed_endpoints = {
  hug = true,
  pat = true,
  poke = true,
  shrug = true,
}

local function api_url(endpoint)
  return "https://kawaii.red/api/gif/" .. endpoint .. "?token=anonymous"
end

function M.fetch_gif(endpoint)
  endpoint = string.lower(tostring(endpoint or ""))
  if not allowed_endpoints[endpoint] then
    error("unsupported kawaii endpoint")
  end

  local payload = bot.http.get_json({
    url = api_url(endpoint),
    max_bytes = 64 * 1024,
  })

  if type(payload) ~= "table" then
    error("kawaii payload is invalid")
  end

  local gif_url = string.match(tostring(payload.response or ""), "^%s*(.-)%s*$")
  if string.sub(string.lower(gif_url), 1, 8) ~= "https://" then
    error("kawaii response must be https")
  end

  return gif_url
end

function M.fetch_gif_or_error(endpoint)
  local ok, result = pcall(M.fetch_gif, endpoint)
  if ok then
    return result, nil
  end

  return nil, shared.reply_embed({
    color = shared.colors.error,
    description = i18n.t("fun.kawaii.error", nil, nil),
  })
end

return M
