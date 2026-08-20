load("@mamacord//api.star", "embed", "embed_field", "reply")

INFO_COLOR = 0x5865F2
ERROR_COLOR = 0xED4245


def timestamp(value):
    if value <= 0:
        return "UNKNOWN"
    return "<t:" + str(value) + ":F>"


def boolean(value):
    if value:
        return "true"
    return "false"


def secure_url(value):
    if type(value) != "string":
        return ""
    normalized = value.strip()
    if normalized.lower().startswith("https://"):
        return "https://" + normalized[8:]
    return ""


def field(ctx, message_id, value):
    return embed_field(
        name=ctx.t(message_id),
        value=str(value),
        inline=True,
    )


def error(ctx, message_id, data={}):
    return reply(
        embeds=[embed(
            description=ctx.t(message_id, data),
            color=ERROR_COLOR,
        )],
        ephemeral=True,
    )
