load("@mamacord//api.star", "channel_option", "embed", "embed_footer", "reply", "subcommand")
load("//lib:shared.star", "INFO_COLOR", "error", "field", "secure_url", "timestamp")


def lookup_channel_handler(ctx):
    if ctx.guild == None:
        return [error(ctx, "err.not_in_guild")]
    channel = ctx.option("channel")
    fields = [
        field(ctx, "info.lookup.channel.field.mention", channel["mention"]),
        field(ctx, "info.lookup.channel.field.type", channel["kind"]),
        field(
            ctx,
            "info.lookup.channel.field.permissions",
            "`" + str(channel["permission_bits"]) + "`",
        ),
        field(ctx, "info.lookup.channel.field.created", timestamp(channel["created_at"])),
    ]
    if channel["parent_id"]:
        fields.append(field(
            ctx,
            "info.lookup.channel.field.parent",
            "<#" + channel["parent_id"] + ">",
        ))
    return [reply(embeds=[embed(
        title=channel["name"],
        color=INFO_COLOR,
        fields=fields,
        footer=embed_footer(text="🆔" + channel["id"]),
    )], ephemeral=True)]


LOOKUP_CHANNEL = subcommand(
    name="channel",
    description="Look up a channel.",
    description_id="cmd.lookup.sub.channel.desc",
    handler=lookup_channel_handler,
    options=[
        channel_option(
            name="channel",
            description="Channel to inspect.",
            description_id="cmd.lookup.sub.channel.opt.channel.desc",
            required=True,
        ),
    ],
)
