load(
    "@mamacord//api.star",
    "attachment_option",
    "create_emoji",
    "delete_emoji",
    "edit_emoji",
    "group",
    "slash_command",
    "string_option",
    "subcommand",
)
load("//lib:parsing.star", "emoji_id", "image_upload_error")
load(
    "//lib:presentation.star",
    "error",
    "guarded",
    "guild_error",
    "response",
)


def emoji_create(ctx):
    guild_failure = guild_error(ctx)
    if guild_failure != None:
        return [guild_failure]
    file = ctx.option("file")
    validation = image_upload_error(
        file,
        "mgr.emojis",
        256 * 1024,
        ["gif", "jpeg", "jpg", "png"],
    )
    if validation != None:
        return [error(ctx, validation[0], validation[1])]
    name = ctx.option("name")
    operation = guarded(
        ctx,
        create_emoji(name=name, attachment_id=file["id"]),
        "mgr.emojis.create_error",
        {"Name": name},
    )
    return [operation, response(ctx, "mgr.emojis.create_success", {"Name": name})]


def emoji_edit(ctx):
    guild_failure = guild_error(ctx)
    if guild_failure != None:
        return [guild_failure]
    raw = ctx.option("emoji")
    emoji = emoji_id(raw)
    if emoji == None:
        return [error(ctx, "mgr.emojis.invalid_emoji", {"Emoji": raw})]
    name = ctx.option("name")
    operation = guarded(
        ctx,
        edit_emoji(emoji=emoji, name=name),
        "mgr.emojis.edit_error",
    )
    return [operation, response(ctx, "mgr.emojis.edit_success", {"Name": name})]


def emoji_delete(ctx):
    guild_failure = guild_error(ctx)
    if guild_failure != None:
        return [guild_failure]
    raw = ctx.option("emoji")
    emoji = emoji_id(raw)
    if emoji == None:
        return [error(ctx, "mgr.emojis.invalid_emoji", {"Emoji": raw})]
    operation = guarded(
        ctx,
        delete_emoji(emoji=emoji),
        "mgr.emojis.delete_error",
    )
    return [operation, response(ctx, "mgr.emojis.delete_success")]


EMOJI_COMMANDS = [group(
    name="emojis",
    description="Manage custom emojis.",
    description_id="cmd.emojis.desc",
    permissions=["manage_expressions", "create_expressions"],
    children=[
        subcommand(
            name="create",
            description="Create emoji.",
            description_id="cmd.emojis.sub.create.desc",
            handler=emoji_create,
            defer="create",
            ephemeral=True,
            options=[
                string_option(
                    name="name",
                    description="Emoji name.",
                    description_id="cmd.emojis.opt.name.desc",
                    required=True,
                    min_length=2,
                    max_length=32,
                ),
                attachment_option(
                    name="file",
                    description="Emoji image.",
                    description_id="cmd.emojis.opt.file.desc",
                    required=True,
                ),
            ],
        ),
        subcommand(
            name="edit",
            description="Rename emoji.",
            description_id="cmd.emojis.sub.edit.desc",
            handler=emoji_edit,
            defer="create",
            ephemeral=True,
            options=[
                string_option(
                    name="emoji",
                    description="Emoji mention or ID.",
                    description_id="cmd.emojis.opt.emoji.desc",
                    required=True,
                    min_length=1,
                    max_length=128,
                ),
                string_option(
                    name="name",
                    description="Emoji name.",
                    description_id="cmd.emojis.opt.name.desc",
                    required=True,
                    min_length=2,
                    max_length=32,
                ),
            ],
        ),
        subcommand(
            name="delete",
            description="Delete emoji.",
            description_id="cmd.emojis.sub.delete.desc",
            handler=emoji_delete,
            defer="create",
            ephemeral=True,
            options=[string_option(
                name="emoji",
                description="Emoji mention or ID.",
                description_id="cmd.emojis.opt.emoji.desc",
                required=True,
                min_length=1,
                max_length=128,
            )],
        ),
    ],
)]
