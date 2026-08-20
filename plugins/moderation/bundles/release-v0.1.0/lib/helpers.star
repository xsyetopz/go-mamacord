load("@mamacord//api.star", "reply")

CONFIG_KEY = "guild_config"
DEFAULT_WARNING_LIMIT = 3
DEFAULT_TIMEOUT_THRESHOLD = 3
DEFAULT_TIMEOUT_MINUTES = 10
UNWARN_TTL_SECONDS = 120


def mention(user_id):
    return "<@" + user_id + ">"


def positive_int(value, fallback):
    if type(value) == "int" and value > 0:
        return value
    return fallback


def config(ctx):
    stored = ctx.state(CONFIG_KEY, {})
    if type(stored) != "dict":
        stored = {}
    return {
        "enabled": (
            stored.get("enabled", True)
            if type(stored.get("enabled", True)) == "bool"
            else True
        ),
        "warning_limit": positive_int(stored.get("warning_limit"), DEFAULT_WARNING_LIMIT),
        "timeout_threshold": positive_int(
            stored.get("timeout_threshold"),
            DEFAULT_TIMEOUT_THRESHOLD,
        ),
        "timeout_minutes": positive_int(stored.get("timeout_minutes"), DEFAULT_TIMEOUT_MINUTES),
    }


def command_error(ctx):
    return reply(content=ctx.t("err.generic"), ephemeral=True)


def guild_error(ctx):
    if ctx.guild != None:
        return None
    return reply(content=ctx.t("err.not_in_guild"), ephemeral=True)


def digits(value):
    if not value:
        return False
    for index in range(len(value)):
        if value[index] not in "0123456789":
            return False
    return True


def parse_flow(raw):
    parts = raw.split("|")
    if len(parts) != 4 or not digits(parts[3]):
        return None
    return {
        "warning_id": parts[0],
        "actor_id": parts[1],
        "target_id": parts[2],
        "issued_at": int(parts[3]),
    }
