load(
    "@mamacord//api.star",
    "reply",
    "row",
    "select",
    "select_option",
    "slash_command",
    "string_option",
    "user_option",
)
load("//lib:helpers.star", "command_error", "config", "guild_error", "mention")


def unwarn(ctx):
    error = guild_error(ctx)
    if error != None:
        return [error]
    if not config(ctx)["enabled"]:
        return [command_error(ctx)]

    target = ctx.option("user")
    if target["id"] == ctx.author["id"]:
        return [reply(content=ctx.t("mod.unwarn.self"), ephemeral=True)]
    if target["bot"] or target["system"]:
        return [reply(content=ctx.t("mod.warn.bot"), ephemeral=True)]

    warnings = ctx.list_warnings(target["id"], limit=25)
    if len(warnings) == 0:
        return [reply(
            content=ctx.t("mod.unwarn.none", {"User": mention(target["id"])}),
            ephemeral=True,
        )]

    options = []
    for warning in warnings:
        label = ("mod " + warning["moderator_id"] + " - " + str(warning["created_at"]))[:100]
        value = "|".join([
            warning["id"],
            ctx.author["id"],
            target["id"],
            str(ctx.now_unix),
        ])
        options.append(select_option(label=label, value=value))

    menu = select(
        handler="unwarn_select",
        kind="string",
        placeholder=ctx.t("mod.unwarn.placeholder"),
        min_values=1,
        max_values=1,
        options=options,
    )
    return [reply(
        content=ctx.t("mod.unwarn.prompt"),
        components=[row(components=[menu])],
        ephemeral=True,
    )]


UNWARN_COMMAND = slash_command(
    name="unwarn",
    description="Remove one warning.",
    description_id="cmd.unwarn.desc",
    handler=unwarn,
    defer="create",
    ephemeral=True,
    permissions=["moderate_members"],
    options=[user_option(
        name="user",
        description="User.",
        description_id="cmd.unwarn.opt.user.desc",
        required=True,
    )],
)
