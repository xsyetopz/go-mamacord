local option = bot.option
local i18n = bot.i18n
local reminders = bot.reminders
local ui = bot.ui

local shared = bot.require("lib/shared.lua")

local LIST_LIMIT = 25

local function create_subcommand()
  return {
    name = "create",
    description = "Create a recurring reminder.",
    description_id = "cmd.remind.sub.create.desc",
    options = {
      option.string("schedule", {
        description = "Cron schedule, like 0 9 * * *.",
        description_id = "cmd.remind.opt.schedule.desc",
        required = true,
        min_length = 1,
        max_length = 128,
      }),
      option.string("kind", {
        description = "What kind of reminder is this?",
        description_id = "cmd.remind.opt.kind.desc",
        required = true,
        choices = shared.reminder_kind_choices,
      }),
      option.string("note", {
        description = "Optional note to include.",
        description_id = "cmd.remind.opt.note.desc",
        max_length = 120,
      }),
      option.string("delivery", {
        description = "Send it to DMs or the current server.",
        description_id = "cmd.remind.opt.delivery.desc",
        choices = shared.delivery_choices,
      }),
      option.channel("channel", {
        description = "Target channel when delivery is channel.",
        description_id = "cmd.remind.opt.channel.desc",
      }),
    },
  }
end

local function list_subcommand()
  return {
    name = "list",
    description = "List your reminders.",
    description_id = "cmd.remind.sub.list.desc",
  }
end

local function delete_subcommand()
  return {
    name = "delete",
    description = "Delete one of your reminders.",
    description_id = "cmd.remind.sub.delete.desc",
    options = {
      option.string("id", {
        description = "Reminder ID to delete directly.",
        description_id = "cmd.remind.opt.id.desc",
      }),
    },
  }
end

local function create_reminder(ctx)
  local config = shared.guild_config(ctx)
  local schedule_text = shared.trim(ctx.command.args.schedule)
  local kind = shared.trim(ctx.command.args.kind)
  local note = shared.trim(ctx.command.args.note)
  local delivery = shared.trim(ctx.command.args.delivery)
  if delivery == "" then
    delivery = "dm"
  end

  local plan = reminders.plan({
    user_id = ctx.user.id,
    schedule = schedule_text,
  })
  if plan == nil then
    return shared.reply_text(i18n.t("wellness.remind.bad_schedule", {
      Schedule = schedule_text,
    }, nil), true)
  end

  local guild_message = nil
  local channel_id = shared.trim(ctx.command.args.channel)
  if delivery == "channel" and ctx.guild.id == "" then
    guild_message = shared.ensure_guild(ctx, i18n.t("err.not_in_guild", nil, nil))
  end
  if guild_message ~= nil then
    return guild_message
  end
  if delivery == "channel" and not config.allow_channel_reminders then
    return shared.reply_text("Channel reminders are disabled in this server.", true)
  end
  if delivery == "channel" and channel_id == "" and config.default_reminder_channel_id ~= "" then
    channel_id = config.default_reminder_channel_id
  end
  if delivery == "channel" and channel_id == "" then
    return shared.reply_text(i18n.t("wellness.remind.channel_required", nil, nil), true)
  end

  local reminder = reminders.create({
    user_id = ctx.user.id,
    schedule = plan.schedule,
    kind = kind,
    note = note,
    delivery = delivery,
    guild_id = ctx.guild.id,
    channel_id = channel_id,
  })
  if reminder == nil then
    return shared.reply_text(i18n.t("err.generic", nil, nil), true)
  end

  return shared.reply_text(i18n.t("wellness.remind.created", {
    ID = reminder.id,
    Kind = reminder.kind,
    NextRun = shared.timestamp(reminder.next_run_at),
    Delivery = reminder.delivery,
  }, nil), true)
end

local function list_reminders(ctx)
  local items = reminders.list(ctx.user.id, LIST_LIMIT)
  if #items == 0 then
    return shared.reply_text(i18n.t("wellness.remind.list.empty", nil, nil), true)
  end

  local lines = {}
  for _, reminder in ipairs(items) do
    table.insert(lines, "- `" .. reminder.id .. "` " .. reminder.kind .. " • " .. shared.timestamp(reminder.next_run_at))
  end

  return shared.reply_text(i18n.t("wellness.remind.list", {
    Lines = table.concat(lines, "\n"),
  }, nil), true)
end

local function delete_reminder(ctx)
  local reminder_id = shared.trim(ctx.command.args.id)
  if reminder_id ~= "" then
    local deleted = reminders.delete(ctx.user.id, reminder_id)
    if not deleted then
      return shared.reply_text(i18n.t("wellness.remind.delete.not_found", nil, nil), true)
    end
    return shared.reply_text(i18n.t("wellness.remind.delete.success", nil, nil), true)
  end

  local items = reminders.list(ctx.user.id, LIST_LIMIT)
  if #items == 0 then
    return shared.reply_text(i18n.t("wellness.remind.list.empty", nil, nil), true)
  end

  local options = {}
  for _, reminder in ipairs(items) do
    table.insert(options, ui.string_option(shared.reminder_option_label(reminder), reminder.id))
  end

  return ui.reply({
    ephemeral = true,
    content = i18n.t("wellness.remind.delete.prompt", nil, nil),
    components = {
      {
        ui.string_select("delete_reminder", {
          placeholder = i18n.t("wellness.remind.delete.placeholder", nil, nil),
          min_values = 1,
          max_values = 1,
          options = options,
        })
      }
    }
  })
end

return bot.command("remind", {
  description = "Create wellness reminders.",
  description_id = "cmd.remind.desc",
  ephemeral = true,
  subcommands = {
    create_subcommand(),
    list_subcommand(),
    delete_subcommand(),
  },
  run = function(ctx)
    local subcommand = shared.trim(ctx.command.subcommand)
    if subcommand == "create" then
      return create_reminder(ctx)
    end
    if subcommand == "list" then
      return list_reminders(ctx)
    end
    if subcommand == "delete" then
      return delete_reminder(ctx)
    end
    return shared.reply_text(i18n.t("err.generic", nil, nil), true)
  end
})
