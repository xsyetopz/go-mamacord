load(
    "@mamacord//api.star",
    "embed",
    "embed_author",
    "embed_footer",
    "reply",
    "string_option",
    "subcommand",
)
load("//lib:shared.star", "INFO_COLOR", "error", "field", "secure_url", "timestamp")


def is_digits(value):
    if not value:
        return False
    for index in range(len(value)):
        if value[index] not in "0123456789":
            return False
    return True


def lookup_guild_handler(ctx):
    guild_id = ctx.option("guild_id")
    if guild_id == None or guild_id == "":
        if ctx.guild == None:
            return [error(ctx, "err.not_in_guild")]
        guild_id = ctx.guild["id"]
    if not is_digits(guild_id):
        return [error(
            ctx,
            "info.lookup.guild.invalid_guild_id",
            {"GuildID": guild_id},
        )]
    if (ctx.guild == None or guild_id != ctx.guild["id"]) and not ctx.is_owner:
        return [error(ctx, "info.lookup.guild.owner_only")]
    guild = ctx.get_guild(guild_id)
    if guild == None:
        return [error(
            ctx,
            "info.lookup.guild.not_accessible",
            {"GuildID": guild_id, "Error": ""},
        )]
    owner = ctx.get_user(guild["owner_id"])
    author = None
    if owner != None:
        author = embed_author(
            name=owner["username"],
            icon_url=secure_url(owner["avatar_url"]),
        )
    fields = [
        field(ctx, "info.lookup.guild.field.roles", guild["roles_count"]),
        field(ctx, "info.lookup.guild.field.emojis", guild["emojis_count"]),
        field(ctx, "info.lookup.guild.field.stickers", guild["stickers_count"]),
        field(ctx, "info.lookup.guild.field.members", guild["member_count"]),
        field(ctx, "info.lookup.guild.field.channels", guild["channels_count"]),
        field(ctx, "info.lookup.guild.field.created", timestamp(guild["created_at"])),
    ]
    return [reply(embeds=[embed(
        title=guild["name"],
        description=guild["description"],
        color=INFO_COLOR,
        fields=fields,
        author=author,
        thumbnail_url=secure_url(guild["icon_url"]),
        image_url=secure_url(guild["banner_url"]),
        footer=embed_footer(text="🆔" + guild_id),
    )])]


LOOKUP_GUILD = subcommand(
    name="guild",
    description="Look up this guild.",
    description_id="cmd.lookup.sub.guild.desc",
    handler=lookup_guild_handler,
    defer="create",
    ephemeral=True,
    options=[
        string_option(
            name="guild_id",
            description="Optional: look up another server by ID (owner only).",
            description_id="cmd.lookup.sub.guild.opt.guild_id.desc",
            min_length=5,
            max_length=32,
        ),
    ],
)
