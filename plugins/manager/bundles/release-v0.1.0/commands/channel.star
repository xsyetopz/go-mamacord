load(
    "@mamacord//api.star",
    "channel_option",
    "group",
    "integer_option",
    "purge_messages",
    "slash_command",
    "string_option",
    "subcommand",
    "set_slowmode",
)
load(
    "//lib:parsing.star",
    "snowflake",
)
load(
    "//lib:presentation.star",
    "error",
    "guarded",
    "guild_error",
    "mention_channel",
    "response",
)


def slowmode(ctx):
    guild_failure = guild_error(ctx)
    if guild_failure != None:
        return [guild_failure]
    channel = ctx.option("channel")
    channel_id = ctx.channel["id"]
    if channel != None:
        channel_id = channel["id"]
    seconds_value = ctx.option("seconds")
    seconds = 0
    if seconds_value != None:
        seconds = seconds_value
    operation = guarded(
        ctx,
        set_slowmode(channel_id=channel_id, seconds=seconds),
        "mgr.slowmode.error",
    )
    message_id = "mgr.slowmode.removed"
    if seconds != 0:
        message_id = "mgr.slowmode.set"
    return [operation, response(
        ctx,
        message_id,
        {"Channel": mention_channel(channel_id), "Seconds": seconds},
    )]


def purge(mode):
    def handle(ctx):
        guild_failure = guild_error(ctx)
        if guild_failure != None:
            return [guild_failure]
        count_value = ctx.option("count")
        count = 2
        if count_value != None:
            count = count_value
        anchor = ""
        raw = ctx.option("message")
        if mode != "all":
            anchor = snowflake(raw)
            if anchor == None:
                return [error(ctx, "mgr.purge.invalid_message")]
        operation = guarded(
            ctx,
            purge_messages(
                channel_id=ctx.channel["id"],
                mode=mode,
                count=count,
                anchor_message_id=anchor,
            ),
            "mgr.purge.error",
        )
        return [operation, response(ctx, "mgr.purge.success", {"Count": "≤" + str(count)})]
    return handle


CHANNEL_COMMANDS = [slash_command(
    name="slowmode",
    description="Set channel slowmode.",
    description_id="cmd.slowmode.desc",
    handler=slowmode,
    defer="create",
    ephemeral=True,
    permissions=["manage_channels"],
    options=[
        channel_option(
            name="channel",
            description="Channel.",
            description_id="cmd.slowmode.opt.channel.desc",
            channel_kinds=["text", "voice", "stage", "forum"],
        ),
        integer_option(
            name="seconds",
            description="Duration.",
            description_id="cmd.slowmode.opt.seconds.desc",
            min_integer=0,
            max_integer=21600,
        ),
    ],
)]


count_all = integer_option(
    name="count",
    description="How many messages.",
    description_id="cmd.purge.opt.count.desc",
    min_integer=2,
    max_integer=100,
)
count_one = integer_option(
    name="count",
    description="How many messages.",
    description_id="cmd.purge.opt.count.desc",
    min_integer=1,
    max_integer=100,
)
message = string_option(
    name="message",
    description="Message ID or link.",
    description_id="cmd.purge.opt.message.desc",
    required=True,
    max_length=255,
)
PURGE_COMMANDS = [group(
    name="purge",
    description="Delete messages in bulk.",
    description_id="cmd.purge.desc",
    permissions=["manage_messages"],
    children=[
        subcommand(
            name="all",
            description="Purge messages.",
            description_id="cmd.purge.sub.all.desc",
            handler=purge("all"),
            defer="create",
            ephemeral=True,
            options=[count_all],
        ),
        subcommand(
            name="before",
            description="Purge before.",
            description_id="cmd.purge.sub.before.desc",
            handler=purge("before"),
            defer="create",
            ephemeral=True,
            options=[message, count_one],
        ),
        subcommand(
            name="after",
            description="Purge after.",
            description_id="cmd.purge.sub.after.desc",
            handler=purge("after"),
            defer="create",
            ephemeral=True,
            options=[message, count_one],
        ),
        subcommand(
            name="around",
            description="Purge around.",
            description_id="cmd.purge.sub.around.desc",
            handler=purge("around"),
            defer="create",
            ephemeral=True,
            options=[message, count_all],
        ),
    ],
)]
