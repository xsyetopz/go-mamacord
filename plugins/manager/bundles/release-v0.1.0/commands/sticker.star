load(
    "@mamacord//api.star",
    "attachment_option",
    "create_sticker",
    "delete_sticker",
    "edit_sticker",
    "group",
    "slash_command",
    "string_option",
    "subcommand",
)
load(
    "//lib:parsing.star",
    "image_upload_error",
    "snowflake",
)
load(
    "//lib:presentation.star",
    "error",
    "guarded",
    "guild_error",
    "response",
)


def sticker_create(ctx):
    guild_failure = guild_error(ctx)
    if guild_failure != None:
        return [guild_failure]
    file = ctx.option("file")
    validation = image_upload_error(
        file,
        "mgr.stickers",
        512 * 1024,
        ["apng", "gif", "png"],
    )
    if validation != None:
        return [error(ctx, validation[0], validation[1])]
    name = ctx.option("name")
    description_value = ctx.option("description")
    description = ""
    if description_value != None:
        description = description_value
    tag = ctx.option("emoji_tag")
    operation = guarded(
        ctx,
        create_sticker(
            name=name,
            description=description,
            emoji_tag=tag,
            attachment_id=file["id"],
        ),
        "mgr.stickers.create_error",
        {"Name": name},
    )
    return [operation, response(ctx, "mgr.stickers.create_success", {"Name": name})]


def sticker_edit(ctx):
    guild_failure = guild_error(ctx)
    if guild_failure != None:
        return [guild_failure]
    raw = ctx.option("id")
    sticker_id = snowflake(raw)
    if sticker_id == None:
        return [error(ctx, "mgr.stickers.invalid_id", {"ID": raw})]
    name = ctx.option("name")
    description = ctx.option("description")
    operation = guarded(
        ctx,
        edit_sticker(sticker_id=sticker_id, name=name, description=description),
        "mgr.stickers.edit_error",
    )
    return [operation, response(ctx, "mgr.stickers.edit_success", {"Name": name})]


def sticker_delete(ctx):
    guild_failure = guild_error(ctx)
    if guild_failure != None:
        return [guild_failure]
    raw = ctx.option("id")
    sticker_id = snowflake(raw)
    if sticker_id == None:
        return [error(ctx, "mgr.stickers.invalid_id", {"ID": raw})]
    operation = guarded(
        ctx,
        delete_sticker(sticker_id=sticker_id),
        "mgr.stickers.delete_error",
    )
    return [operation, response(ctx, "mgr.stickers.delete_success")]


STICKER_COMMANDS = [group(
    name="stickers",
    description="Manage server stickers.",
    description_id="cmd.stickers.desc",
    permissions=["manage_expressions", "create_expressions"],
    children=[
        subcommand(
            name="create",
            description="Create sticker.",
            description_id="cmd.stickers.sub.create.desc",
            handler=sticker_create,
            defer="create",
            ephemeral=True,
            options=[
                string_option(
                    name="name",
                    description="Sticker name.",
                    description_id="cmd.stickers.opt.name.desc",
                    required=True,
                    min_length=2,
                    max_length=30,
                ),
                string_option(
                    name="emoji_tag",
                    description="Emoji tag.",
                    description_id="cmd.stickers.opt.emoji_tag.desc",
                    required=True,
                    min_length=1,
                    max_length=64,
                ),
                attachment_option(
                    name="file",
                    description="Sticker file.",
                    description_id="cmd.stickers.opt.file.desc",
                    required=True,
                ),
                string_option(
                    name="description",
                    description="Description.",
                    description_id="cmd.stickers.opt.description.desc",
                    min_length=2,
                    max_length=100,
                ),
            ],
        ),
        subcommand(
            name="edit",
            description="Edit sticker.",
            description_id="cmd.stickers.sub.edit.desc",
            handler=sticker_edit,
            defer="create",
            ephemeral=True,
            options=[
                string_option(
                    name="id",
                    description="Sticker ID or link.",
                    description_id="cmd.stickers.opt.id.desc",
                    required=True,
                    min_length=1,
                    max_length=255,
                ),
                string_option(
                    name="name",
                    description="Sticker name.",
                    description_id="cmd.stickers.opt.name.desc",
                    required=True,
                    min_length=2,
                    max_length=30,
                ),
                string_option(
                    name="description",
                    description="Description.",
                    description_id="cmd.stickers.opt.description.desc",
                    min_length=2,
                    max_length=100,
                ),
            ],
        ),
        subcommand(
            name="delete",
            description="Delete sticker.",
            description_id="cmd.stickers.sub.delete.desc",
            handler=sticker_delete,
            defer="create",
            ephemeral=True,
            options=[string_option(
                name="id",
                description="Sticker ID or link.",
                description_id="cmd.stickers.opt.id.desc",
                required=True,
                min_length=1,
                max_length=255,
            )],
        ),
    ],
)]
