load("@mamacord//api.star", "embed", "embed_footer", "reply", "subcommand", "user_option")
load("//lib:shared.star", "INFO_COLOR", "boolean", "error", "field", "secure_url", "timestamp")


def lookup_user_handler(ctx):
    selected = ctx.option("user")
    user_id = ctx.author["id"] if selected == None else selected["id"]
    target = ctx.get_user(user_id)
    if target == None:
        return [error(ctx, "info.lookup.user.error")]
    member = ctx.get_member(user_id) if ctx.guild != None else None
    color = INFO_COLOR
    if target["accent_color"] != None and target["accent_color"] != 0:
        color = target["accent_color"]
    fields = [
        field(ctx, "info.lookup.user.field.bot", boolean(target["bot"])),
        field(ctx, "info.lookup.user.field.system", boolean(target["system"])),
        field(ctx, "info.lookup.user.field.locale", ctx.locale),
        field(ctx, "info.lookup.user.field.created", timestamp(target["created_at"])),
    ]
    if member != None and member["joined_at"] > 0:
        fields.append(field(
            ctx,
            "info.lookup.user.field.joined",
            timestamp(member["joined_at"]),
        ))
    if member != None and len(member["role_ids"]) > 0:
        fields.append(field(
            ctx,
            "info.lookup.user.field.roles",
            len(member["role_ids"]),
        ))
    return [reply(embeds=[embed(
        title=target["name"],
        color=color,
        fields=fields,
        thumbnail_url=secure_url(target["avatar_url"]),
        image_url=secure_url(target["banner_url"]),
        footer=embed_footer(text="🆔" + user_id),
    )])]


LOOKUP_USER = subcommand(
    name="user",
    description="Look up a user.",
    description_id="cmd.lookup.sub.user.desc",
    handler=lookup_user_handler,
    defer="create",
    ephemeral=True,
    options=[
        user_option(
            name="user",
            description="User to inspect.",
            description_id="cmd.lookup.sub.user.opt.user.desc",
        ),
    ],
)
