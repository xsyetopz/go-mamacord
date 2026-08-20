load(
    "@mamacord//api.star",
    "set_nickname",
    "slash_command",
    "string_option",
    "user_option",
)
load(
    "//lib:presentation.star",
    "error",
    "guarded",
    "guild_error",
    "mention_user",
    "response",
)


def nickname(ctx):
    guild_failure = guild_error(ctx)
    if guild_failure != None:
        return [guild_failure]
    user = ctx.option("user")
    if user["id"] == ctx.author["id"]:
        return [error(ctx, "mgr.nick.self_error")]
    if user["bot"] or user["system"]:
        return [error(ctx, "mod.warn.bot")]
    nickname_value = ctx.option("nickname")
    nickname = ""
    if nickname_value != None:
        nickname = nickname_value.strip()
    operation = guarded(
        ctx,
        set_nickname(user_id=user["id"], nickname=nickname),
        "mgr.nick.error",
        {"User": mention_user(user["id"]), "Nickname": nickname},
    )
    message_id = "mgr.nick.reset"
    if nickname != "":
        message_id = "mgr.nick.set"
    return [operation, response(
        ctx,
        message_id,
        {"User": mention_user(user["id"]), "Nickname": nickname},
    )]


NICKNAME_COMMANDS = [slash_command(
    name="nick",
    description="Change nickname.",
    description_id="cmd.nick.desc",
    handler=nickname,
    defer="create",
    ephemeral=True,
    permissions=["manage_nicknames"],
    options=[
        user_option(
            name="user",
            description="User.",
            description_id="cmd.nick.opt.user.desc",
            required=True,
        ),
        string_option(
            name="nickname",
            description="Nickname.",
            description_id="cmd.nick.opt.nickname.desc",
            min_length=1,
            max_length=32,
        ),
    ],
)]
