load(
    "@mamacord//api.star",
    "button",
    "embed",
    "kv_put",
    "reply",
    "row",
    "update",
)

COUNTER_KEY = "counter"
DIGITS = {
    "0": 0,
    "1": 1,
    "2": 2,
    "3": 3,
    "4": 4,
    "5": 5,
    "6": 6,
    "7": 7,
    "8": 8,
    "9": 9,
}


def counter(ctx):
    value = ctx.state(COUNTER_KEY, 0)
    if type(value) != "int":
        return 0
    return value


def counter_effect(ctx, value):
    return kv_put(
        key=COUNTER_KEY,
        value=value,
        expected_version=ctx.state_version(COUNTER_KEY),
    )


def controls():
    return [row(components=[
        button(handler="inc", label="Increment", style="primary"),
        button(handler="set", label="Set...", style="secondary"),
    ])]


def render_counter(ctx, value, update_message=False):
    content = ctx.t("example.counter", {"Count": value})
    if update_message:
        return update(content=content, components=controls())
    return reply(content=content, components=controls(), ephemeral=True)


def guild_error(ctx, update_message=False):
    if ctx.guild != None:
        return None
    presentation = embed(
        title=ctx.t("example.not_in_guild.title"),
        description=ctx.t("example.not_in_guild.body"),
    )
    if update_message:
        return update(content="", embeds=[presentation], components=[])
    return reply(embeds=[presentation], ephemeral=True)


def parse_int(raw):
    if type(raw) != "string" or len(raw) == 0:
        return None
    negative = raw[0] == "-"
    digits = raw[1:] if negative else raw
    if len(digits) == 0:
        return None
    value = 0
    for index in range(len(digits)):
        digit = digits[index]
        if digit not in DIGITS:
            return None
        value = value * 10 + DIGITS[digit]
    if negative:
        value = -value
    if value < -9223372036854775808 or value > 9223372036854775807:
        return None
    return value
