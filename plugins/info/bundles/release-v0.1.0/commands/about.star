load("@mamacord//api.star", "embed", "embed_author", "embed_footer", "reply", "slash_command")
load("//lib:shared.star", "INFO_COLOR", "secure_url")


def about_handler(ctx):
    runtime = ctx.runtime
    bot_user = ctx.bot_user
    author = None
    if bot_user != None:
        name = bot_user["username"]
        if runtime["version"]:
            name += " " + runtime["version"]
        author = embed_author(
            name=name,
            icon_url=secure_url(bot_user["avatar_url"]),
        )
    footer = None
    if ctx.author != None:
        footer = embed_footer(
            text=ctx.author["username"],
            icon_url=secure_url(ctx.author["avatar_url"]),
        )
    return [reply(
        embeds=[embed(
            title=ctx.t("info.about.title", {"Version": runtime["version"]}),
            description=runtime["description"],
            url=secure_url(runtime["repository"]),
            color=INFO_COLOR,
            author=author,
            footer=footer,
            image_url=secure_url(runtime["mascot_image_url"]),
        )],
        ephemeral=True,
    )]


ABOUT = slash_command(
    name="about",
    description="Show bot details.",
    description_id="cmd.about.desc",
    handler=about_handler,
    ephemeral=True,
)
