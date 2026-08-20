load("@mamacord//api.star", "reply")

CONFIG_KEY = "guild_config"
KINDS = ["hydrate", "stretch", "breathe", "meds", "sleep", "checkin"]


def timestamp(value):
    return "<t:" + str(value) + ":f>"


def config(ctx):
    stored = ctx.state(CONFIG_KEY, {})
    if type(stored) != "dict":
        stored = {}
    enabled = stored.get("enabled", True)
    allow_channel_reminders = stored.get("allow_channel_reminders", True)
    default_channel = stored.get("default_reminder_channel_id", "")
    return {
        "enabled": enabled if type(enabled) == "bool" else True,
        "allow_channel_reminders": (
            allow_channel_reminders
            if type(allow_channel_reminders) == "bool"
            else True
        ),
        "default_reminder_channel_id": default_channel if type(default_channel) == "string" else "",
    }


def generic(ctx):
    return reply(content=ctx.t("err.generic"), ephemeral=True)


def reminder_id(ctx):
    return ctx.new_id()
