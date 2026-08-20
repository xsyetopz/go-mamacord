load("@mamacord//api.star", "reply")
load("//lib:shared.star", "ERROR_COLOR", "ENDPOINT_EMOJI", "embed_reply", "mention")


def guild_error(ctx):
    if ctx.guild != None:
        return None
    return reply(content=ctx.t("err.not_in_guild"), ephemeral=True)


def fetch_gif(ctx, endpoint):
    payload = ctx.http_get_json(
        url="https://kawaii.red/api/gif/" + endpoint + "?token=anonymous",
        max_bytes=65536,
    )
    if payload == None or type(payload) != "dict":
        return None
    gif_url = payload.get("response", "").strip()
    if not gif_url.startswith("https://"):
        return None
    return gif_url


def kawaii_handler(endpoint):
    def handle(ctx):
        error = guild_error(ctx)
        if error != None:
            return [error]
        target = ctx.option("user")
        if target["id"] == ctx.author["id"]:
            return [embed_reply(
                description=ctx.t("fun.kawaii.self_error"),
                color=ERROR_COLOR,
                ephemeral=True,
            )]
        gif_url = fetch_gif(ctx, endpoint)
        if gif_url == None:
            return [embed_reply(
                description=ctx.t("fun.kawaii.error"),
                color=ERROR_COLOR,
            )]
        return [embed_reply(
            description=ctx.t(
                "fun.kawaii.user_mention_only",
                {"Emoji": ENDPOINT_EMOJI[endpoint], "User": mention(target["id"])},
            ),
            image_url=gif_url,
            footer=ctx.t("fun.kawaii.footer"),
        )]
    return handle
