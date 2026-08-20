load(
    "@mamacord//api.star",
    "attempt",
    "clear_timezone",
    "group",
    "reply",
    "set_timezone",
    "slash_command",
    "string_option",
    "subcommand",
)
load("//lib:helpers.star", "generic")


def timezone_set(ctx):
    raw = ctx.option("iana").strip()
    normalized = ctx.normalize_timezone(raw)
    if normalized == None:
        return [reply(
            content=ctx.t("wellness.timezone.invalid", {"Timezone": raw}),
            ephemeral=True,
        )]
    return [
        attempt(
            effect=set_timezone(timezone=normalized),
            on_error=generic(ctx),
        ),
        reply(
            content=ctx.t("wellness.timezone.set", {"Timezone": normalized}),
            ephemeral=True,
        ),
    ]


def timezone_show(ctx):
    settings = ctx.user_settings()
    name = "" if settings == None else settings["timezone"]
    if not name:
        return [reply(content=ctx.t("wellness.timezone.unset"), ephemeral=True)]
    return [reply(
        content=ctx.t("wellness.timezone.show", {"Timezone": name}),
        ephemeral=True,
    )]


def timezone_clear(ctx):
    return [
        attempt(effect=clear_timezone(), on_error=generic(ctx)),
        reply(content=ctx.t("wellness.timezone.cleared"), ephemeral=True),
    ]


def timezone_command():
    return group(
        name="timezone",
        description="Manage your timezone.",
        description_id="cmd.timezone.desc",
        children=[
            subcommand(
                name="set",
                description="Save timezone.",
                description_id="cmd.timezone.sub.set.desc",
                handler=timezone_set,
                defer="create",
                ephemeral=True,
                options=[string_option(
                    name="iana",
                    description="IANA timezone.",
                    description_id="cmd.timezone.opt.iana.desc",
                    required=True,
                    min_length=1,
                    max_length=64,
                )],
            ),
            subcommand(
                name="show",
                description="Show timezone.",
                description_id="cmd.timezone.sub.show.desc",
                handler=timezone_show,
                defer="create",
                ephemeral=True,
            ),
            subcommand(
                name="clear",
                description="Clear timezone.",
                description_id="cmd.timezone.sub.clear.desc",
                handler=timezone_clear,
                defer="create",
                ephemeral=True,
            ),
        ],
    )


TIMEZONE_COMMAND = timezone_command()
