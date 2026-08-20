load("@mamacord//api.star", "slash_command", "string_option", "user_option")
load("//lib:kawaii.star", "fetch_gif", "guild_error", "kawaii_handler")
load("//lib:shared.star", "ERROR_COLOR", "embed_reply")


def shrug_handler(ctx):
    error = guild_error(ctx)
    if error != None:
        return [error]
    gif_url = fetch_gif(ctx, "shrug")
    if gif_url == None:
        return [embed_reply(
            description=ctx.t("fun.kawaii.error"),
            color=ERROR_COLOR,
        )]
    message = ctx.option("message")
    content = ""
    if message != None:
        content = message
    return [embed_reply(
        content=content,
        image_url=gif_url,
        footer=ctx.t("fun.kawaii.footer"),
    )]


HUG = slash_command(
    name="hug",
    defer="create",
    description="Give someone a warm hug.",
    description_id="cmd.hug.desc",
    handler=kawaii_handler("hug"),
    options=[user_option(
        name="user",
        description="Target user",
        description_id="cmd.hug.opt.user.desc",
        required=True,
    )],
)
PAT = slash_command(
    name="pat",
    defer="create",
    description="Give someone a gentle head-pat.",
    description_id="cmd.pat.desc",
    handler=kawaii_handler("pat"),
    options=[user_option(
        name="user",
        description="Target user",
        description_id="cmd.pat.opt.user.desc",
        required=True,
    )],
)
POKE = slash_command(
    name="poke",
    defer="create",
    description="Give someone a tiny poke.",
    description_id="cmd.poke.desc",
    handler=kawaii_handler("poke"),
    options=[user_option(
        name="user",
        description="Target user",
        description_id="cmd.poke.opt.user.desc",
        required=True,
    )],
)
SHRUG = slash_command(
    name="shrug",
    defer="create",
    description="Send a shrug.",
    description_id="cmd.shrug.desc",
    handler=shrug_handler,
    options=[string_option(
        name="message",
        description="Anything to add?",
        description_id="cmd.shrug.opt.message.desc",
        max_length=2000,
    )],
)
