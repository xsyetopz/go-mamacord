local option = bot.option
local i18n = bot.i18n
local warnings = bot.warnings
local audit = bot.audit
local time = bot.time

local shared = bot.require("lib/shared.lua")

return bot.command("warn", {
  description = "Warn a member.",
  description_id = "cmd.warn.desc",
  ephemeral = true,
  default_member_permissions = { "moderate_members" },
  options = {
    option.user("user", {
      description = "User to warn.",
      description_id = "cmd.warn.opt.user.desc",
      required = true,
    }),
    option.string("reason", {
      description = "Reason for the warning.",
      description_id = "cmd.warn.opt.reason.desc",
      required = true,
      min_length = 1,
      max_length = 255,
    }),
  },
  run = function(ctx)
    local guild_error = shared.ensure_guild(ctx, i18n.t("err.not_in_guild", nil, nil))
    if guild_error ~= nil then
      return guild_error
    end

    local target_id = shared.trim(ctx.command.args.user)
    local resolved = shared.resolved_user(ctx, "user")
    local reason = shared.trim(ctx.command.args.reason)
    local config = shared.guild_config(ctx)

    if target_id == ctx.user.id then
      return shared.reply_text(i18n.t("mod.warn.self", nil, nil), true)
    end
    if resolved.bot or resolved.system then
      return shared.reply_text(i18n.t("mod.warn.bot", nil, nil), true)
    end
    if reason == "" then
      return shared.reply_text(i18n.t("err.generic", nil, nil), true)
    end

    local count = warnings.count(ctx.guild.id, target_id)
    if count >= config.warning_limit then
      return shared.reply_text(i18n.t("mod.warn.too_many", {
        User = shared.mention(target_id),
      }, nil), true)
    end

    local now = time.unix()
    warnings.create({
      guild_id = ctx.guild.id,
      user_id = target_id,
      moderator_id = ctx.user.id,
      reason = reason,
      created_at = now,
    })

    audit.append({
      guild_id = ctx.guild.id,
      actor_id = ctx.user.id,
      action = "warn.create",
      target_type = "user",
      target_id = target_id,
      created_at = now,
      meta_json = "{}",
    })

    local timeout_minutes = 0
    local timeout_failed = false
    if count + 1 >= config.timeout_threshold then
      local timeout_seconds = config.timeout_minutes * 60
      local until_unix = now + timeout_seconds
      local timed_out = false
      timed_out, _ = shared.timeout_member(ctx.guild.id, target_id, until_unix)
      if timed_out then
        timeout_minutes = config.timeout_minutes
        audit.append({
          guild_id = ctx.guild.id,
          actor_id = ctx.user.id,
          action = "warn.timeout",
          target_type = "user",
          target_id = target_id,
          created_at = now,
          meta_json = "{\"until\":" .. tostring(until_unix) .. "}",
        })
      else
        timeout_failed = true
      end
    end

    shared.send_dm(target_id, {
      content = i18n.t("mod.warn.dm", {
        Reason = reason,
        TimeoutMinutes = timeout_minutes,
      }, nil),
    })

    return shared.reply_text(i18n.t("mod.warn.success", {
      User = shared.mention(target_id),
      Reason = reason,
      TimeoutMinutes = timeout_minutes,
      TimeoutFailed = timeout_failed,
    }, nil), true)
  end
})
