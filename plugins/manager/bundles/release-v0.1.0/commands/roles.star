load(
    "@mamacord//api.star",
    "add_role",
    "boolean_option",
    "create_role",
    "delete_role",
    "edit_role",
    "group",
    "remove_role",
    "role_option",
    "slash_command",
    "string_option",
    "subcommand",
    "user_option",
)
load(
    "//lib:parsing.star",
    "parse_hex",
)
load(
    "//lib:presentation.star",
    "error",
    "guarded",
    "guild_error",
    "mention_user",
    "response",
)


def role_create(ctx):
    guild_failure = guild_error(ctx)
    if guild_failure != None:
        return [guild_failure]
    name = ctx.option("name")
    color = parse_hex(ctx.option("colour"))
    if color == "invalid":
        return [error(ctx, "mgr.roles.invalid_colour", {"Colour": ctx.option("colour")})]
    operation = guarded(
        ctx,
        create_role(
            name=name,
            color=color,
            hoist=ctx.option("hoist"),
            mentionable=ctx.option("mentionable"),
        ),
        "mgr.roles.create_error",
        {"Name": name},
    )
    return [
        operation,
        response(
            ctx,
            "mgr.roles.create_success",
            {"Role": "`" + name + "`", "Name": name},
        ),
    ]


def role_edit(ctx):
    guild_failure = guild_error(ctx)
    if guild_failure != None:
        return [guild_failure]
    role = ctx.option("role")
    if role["id"] == ctx.guild["id"]:
        return [error(ctx, "mgr.roles.cannot_edit_everyone")]
    color = parse_hex(ctx.option("colour"))
    if color == "invalid":
        return [error(ctx, "mgr.roles.invalid_colour", {"Colour": ctx.option("colour")})]
    operation = guarded(
        ctx,
        edit_role(
            role_id=role["id"],
            name=ctx.option("name"),
            color=color,
            hoist=ctx.option("hoist"),
            mentionable=ctx.option("mentionable"),
        ),
        "mgr.roles.edit_error",
        {"Role": role["mention"]},
    )
    return [operation, response(ctx, "mgr.roles.edit_success", {"Role": role["mention"]})]


def role_delete(ctx):
    guild_failure = guild_error(ctx)
    if guild_failure != None:
        return [guild_failure]
    role = ctx.option("role")
    if role["id"] == ctx.guild["id"]:
        return [error(ctx, "mgr.roles.cannot_delete_everyone")]
    operation = guarded(
        ctx,
        delete_role(role_id=role["id"]),
        "mgr.roles.delete_error",
        {"Role": role["mention"]},
    )
    return [operation, response(ctx, "mgr.roles.delete_success", {"Name": role["name"]})]


def member_role(add):
    def handle(ctx):
        guild_failure = guild_error(ctx)
        if guild_failure != None:
            return [guild_failure]
        role = ctx.option("role")
        member = ctx.option("member")
        effect = None
        if add:
            effect = add_role(user_id=member["id"], role_id=role["id"])
        else:
            effect = remove_role(user_id=member["id"], role_id=role["id"])
        action = "add"
        if not add:
            action = "remove"
        data = {
            "Role": role["mention"],
            "User": mention_user(member["id"]),
        }
        return [
            guarded(ctx, effect, "mgr.roles." + action + "_error", data),
            response(ctx, "mgr.roles." + action + "_success", data),
        ]
    return handle


deferred = "create"
role_name = string_option(
    name="name",
    description="Role name.",
    description_id="cmd.roles.opt.name.desc",
    required=True,
    min_length=1,
    max_length=100,
)
role_colour = string_option(
    name="colour",
    description="Hex role colour.",
    description_id="cmd.roles.opt.colour.desc",
    min_length=6,
    max_length=7,
)
role_hoist = boolean_option(
    name="hoist",
    description="Show separately.",
    description_id="cmd.roles.opt.hoist.desc",
)
role_mentionable = boolean_option(
    name="mentionable",
    description="Allow mentions.",
    description_id="cmd.roles.opt.mentionable.desc",
)
roles = group(
    name="roles",
    description="Manage roles.",
    description_id="cmd.roles.desc",
    permissions=["manage_roles"],
    children=[
        subcommand(
            name="create",
            description="Create a new role.",
            description_id="cmd.roles.sub.create.desc",
            handler=role_create,
            defer=deferred,
            ephemeral=True,
            options=[role_name, role_colour, role_hoist, role_mentionable],
        ),
        subcommand(
            name="edit",
            description="Edit a role.",
            description_id="cmd.roles.sub.edit.desc",
            handler=role_edit,
            defer=deferred,
            ephemeral=True,
            options=[
                role_option(
                    name="role",
                    description="Role to edit.",
                    description_id="cmd.roles.opt.role.desc",
                    required=True,
                ),
                string_option(
                    name="name",
                    description="New role name.",
                    description_id="cmd.roles.opt.name.desc",
                    min_length=1,
                    max_length=100,
                ),
                role_colour,
                role_hoist,
                role_mentionable,
            ],
        ),
        subcommand(
            name="delete",
            description="Delete a role.",
            description_id="cmd.roles.sub.delete.desc",
            handler=role_delete,
            defer=deferred,
            ephemeral=True,
            options=[role_option(
                name="role",
                description="Role to delete.",
                description_id="cmd.roles.opt.role.desc",
                required=True,
            )],
        ),
        subcommand(
            name="add",
            description="Add role to member.",
            description_id="cmd.roles.sub.add.desc",
            handler=member_role(True),
            defer=deferred,
            ephemeral=True,
            options=[
                role_option(
                    name="role",
                    description="Role.",
                    description_id="cmd.roles.opt.role.desc",
                    required=True,
                ),
                user_option(
                    name="member",
                    description="Member.",
                    description_id="cmd.roles.opt.member.desc",
                    required=True,
                ),
            ],
        ),
        subcommand(
            name="remove",
            description="Remove role from member.",
            description_id="cmd.roles.sub.remove.desc",
            handler=member_role(False),
            defer=deferred,
            ephemeral=True,
            options=[
                role_option(
                    name="role",
                    description="Role.",
                    description_id="cmd.roles.opt.role.desc",
                    required=True,
                ),
                user_option(
                    name="member",
                    description="Member.",
                    description_id="cmd.roles.opt.member.desc",
                    required=True,
                ),
            ],
        ),
    ],
)
ROLE_COMMANDS = [
    roles,
]
