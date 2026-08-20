load(
    "@mamacord//api.star",
    "create_checkin",
    "attempt",
    "integer_option",
    "reply",
    "slash_command",
    "boolean_option",
)
load("//lib:helpers.star", "generic", "timestamp")


def checkin(ctx):
    mood = ctx.option("mood")
    history = ctx.option("history") == True
    if history and mood == None:
        entries = ctx.list_checkins(limit=10)
        if len(entries) == 0:
            return [reply(content=ctx.t("wellness.checkin.history.empty"), ephemeral=True)]
        lines = []
        for entry in entries:
            lines.append("- " + timestamp(entry["created_at"]) + ": " + str(entry["mood"]) + "/5")
        return [reply(
            content=ctx.t("wellness.checkin.history", {"Lines": "\n".join(lines)}),
            ephemeral=True,
        )]
    if mood == None:
        return [reply(content=ctx.t("wellness.checkin.prompt"), ephemeral=True)]

    operation = attempt(
        effect=create_checkin(mood=mood, created_at=ctx.now_unix),
        on_error=generic(ctx),
    )
    return [
        operation,
        reply(
            content=ctx.t("wellness.checkin.saved", {"Mood": mood}),
            ephemeral=True,
        ),
    ]


CHECKIN_COMMAND = slash_command(
    name="checkin",
    description="Save how you feel.",
    description_id="cmd.checkin.desc",
    handler=checkin,
    defer="create",
    ephemeral=True,
    options=[
        integer_option(
            name="mood",
            description="Mood 1 to 5.",
            description_id="cmd.checkin.opt.mood.desc",
            min_integer=1,
            max_integer=5,
        ),
        boolean_option(
            name="history",
            description="Show history.",
            description_id="cmd.checkin.opt.history.desc",
        ),
    ],
)
