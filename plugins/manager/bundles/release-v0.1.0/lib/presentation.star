load(
    "@mamacord//api.star",
    "attempt",
    "embed",
    "reply",
)

SUCCESS_COLOR = 0x57F287
ERROR_COLOR = 0xED4245


def response(ctx, message_id, data={}, success=True):
    color = SUCCESS_COLOR
    if not success:
        color = ERROR_COLOR
    return reply(embeds=[embed(
        description=ctx.t(message_id, data),
        color=color,
    )])


def error(ctx, message_id, data={}):
    return response(ctx, message_id, data, success=False)


def guild_error(ctx):
    if ctx.guild != None:
        return None
    return reply(content=ctx.t("err.not_in_guild"))


def guarded(ctx, effect, error_id, error_data={}):
    return attempt(
        effect=effect,
        on_error=error(ctx, error_id, error_data),
    )


def mention_user(user_id):
    return "<@" + user_id + ">"


def mention_role(role_id):
    return "<@&" + role_id + ">"


def mention_channel(channel_id):
    return "<#" + channel_id + ">"
