load("@mamacord//api.star", "embed", "embed_footer", "reply", "role_option", "subcommand")
load("//lib:shared.star", "boolean", "error", "field", "timestamp")


def lookup_role_handler(ctx):
    if ctx.guild == None:
        return [error(ctx, "err.not_in_guild")]
    role = ctx.option("role")
    fields = [
        field(ctx, "info.lookup.role.field.mention", role["mention"]),
        field(ctx, "info.lookup.role.field.position", role["position"]),
        field(ctx, "info.lookup.role.field.hoist", boolean(role["hoist"])),
        field(ctx, "info.lookup.role.field.mentionable", boolean(role["mentionable"])),
        field(ctx, "info.lookup.role.field.managed", boolean(role["managed"])),
        field(ctx, "info.lookup.role.field.permissions", "`" + str(role["permission_bits"]) + "`"),
        field(ctx, "info.lookup.role.field.created", timestamp(role["created_at"])),
    ]
    return [reply(embeds=[embed(
        title=role["name"],
        color=role["color"],
        fields=fields,
        footer=embed_footer(text="🆔" + role["id"]),
    )], ephemeral=True)]


LOOKUP_ROLE = subcommand(
    name="role",
    description="Look up a role.",
    description_id="cmd.lookup.sub.role.desc",
    handler=lookup_role_handler,
    options=[
        role_option(
            name="role",
            description="Role to inspect.",
            description_id="cmd.lookup.sub.role.opt.role.desc",
            required=True,
        ),
    ],
)
