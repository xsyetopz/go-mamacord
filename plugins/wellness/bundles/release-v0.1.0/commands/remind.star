load(
    "@mamacord//api.star",
    "attempt",
    "channel_option",
    "choice",
    "create_reminder",
    "delete_reminder",
    "group",
    "reply",
    "row",
    "select",
    "select_option",
    "slash_command",
    "string_option",
    "subcommand",
)
load("//lib:helpers.star", "KINDS", "config", "generic", "reminder_id", "timestamp")


def remind_create(ctx):
    settings = config(ctx)
    schedule = ctx.option("schedule").strip()
    plan = ctx.plan_reminder(schedule)
    if plan == None:
        return [reply(
            content=ctx.t("wellness.remind.bad_schedule", {"Schedule": schedule}),
            ephemeral=True,
        )]

    kind = ctx.option("kind")
    note_value = ctx.option("note")
    note = "" if note_value == None else note_value.strip()
    delivery_value = ctx.option("delivery")
    delivery = "dm" if delivery_value == None else delivery_value
    channel = ctx.option("channel")
    channel_id = "" if channel == None or delivery != "channel" else channel["id"]
    if delivery == "channel":
        if ctx.guild == None:
            return [reply(content=ctx.t("err.not_in_guild"), ephemeral=True)]
        if not settings["allow_channel_reminders"]:
            return [reply(content="Channel reminders are disabled in this server.", ephemeral=True)]
        if not channel_id:
            channel_id = settings["default_reminder_channel_id"]
        if not channel_id:
            return [reply(content=ctx.t("wellness.remind.channel_required"), ephemeral=True)]

    reminder = reminder_id(ctx)
    operation = attempt(
        effect=create_reminder(
            reminder_id=reminder,
            schedule=plan["schedule"],
            kind=kind,
            note=note,
            delivery=delivery,
            channel_id=channel_id,
            next_run_at=plan["next_run_at"],
        ),
        on_error=generic(ctx),
    )
    return [
        operation,
        reply(
            content=ctx.t("wellness.remind.created", {
                "ID": reminder,
                "Kind": kind,
                "NextRun": timestamp(plan["next_run_at"]),
                "Delivery": delivery,
            }),
            ephemeral=True,
        ),
    ]


def remind_list(ctx):
    items = ctx.list_reminders(limit=25)
    if len(items) == 0:
        return [reply(content=ctx.t("wellness.remind.list.empty"), ephemeral=True)]
    lines = []
    for item in items:
        lines.append(
            "- `" + item["id"] + "` " + item["kind"] + " • " + timestamp(item["next_run_at"]),
        )
    return [reply(
        content=ctx.t("wellness.remind.list", {"Lines": "\n".join(lines)}),
        ephemeral=True,
    )]


def remind_delete(ctx):
    reminder = ctx.option("id")
    if reminder != None and reminder.strip():
        return [
            attempt(
                effect=delete_reminder(reminder_id=reminder.strip()),
                on_error=reply(content=ctx.t("wellness.remind.delete.not_found"), ephemeral=True),
            ),
            reply(content=ctx.t("wellness.remind.delete.success"), ephemeral=True),
        ]

    items = ctx.list_reminders(limit=25)
    if len(items) == 0:
        return [reply(content=ctx.t("wellness.remind.list.empty"), ephemeral=True)]
    options = []
    for item in items:
        options.append(select_option(
            label=(item["kind"] + " • " + timestamp(item["next_run_at"]))[:90],
            value=item["id"],
        ))
    menu = select(
        handler="delete_reminder",
        kind="string",
        placeholder=ctx.t("wellness.remind.delete.placeholder"),
        min_values=1,
        max_values=1,
        options=options,
    )
    return [reply(
        content=ctx.t("wellness.remind.delete.prompt"),
        components=[row(components=[menu])],
        ephemeral=True,
    )]


def reminder_command():
    kinds = [choice(name=value, value=value) for value in KINDS]
    deliveries = [
        choice(name="dm", value="dm"),
        choice(name="channel", value="channel"),
    ]
    return group(
        name="remind",
        description="Create wellness reminders.",
        description_id="cmd.remind.desc",
        children=[
            subcommand(
                name="create",
                description="Create recurring reminder.",
                description_id="cmd.remind.sub.create.desc",
                handler=remind_create,
                defer="create",
                ephemeral=True,
                options=[
                    string_option(
                        name="schedule",
                        description="Cron schedule.",
                        description_id="cmd.remind.opt.schedule.desc",
                        required=True,
                        min_length=1,
                        max_length=128,
                    ),
                    string_option(
                        name="kind",
                        description="Reminder kind.",
                        description_id="cmd.remind.opt.kind.desc",
                        required=True,
                        choices=kinds,
                    ),
                    string_option(
                        name="note",
                        description="Optional note.",
                        description_id="cmd.remind.opt.note.desc",
                        max_length=120,
                    ),
                    string_option(
                        name="delivery",
                        description="Delivery.",
                        description_id="cmd.remind.opt.delivery.desc",
                        choices=deliveries,
                    ),
                    channel_option(
                        name="channel",
                        description="Target channel.",
                        description_id="cmd.remind.opt.channel.desc",
                    ),
                ],
            ),
            subcommand(
                name="list",
                description="List reminders.",
                description_id="cmd.remind.sub.list.desc",
                handler=remind_list,
                defer="create",
                ephemeral=True,
            ),
            subcommand(
                name="delete",
                description="Delete reminder.",
                description_id="cmd.remind.sub.delete.desc",
                handler=remind_delete,
                defer="create",
                ephemeral=True,
                options=[string_option(
                    name="id",
                    description="Reminder ID.",
                    description_id="cmd.remind.opt.id.desc",
                    max_length=100,
                )],
            ),
        ],
    )


REMINDER_COMMAND = reminder_command()
