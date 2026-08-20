load(
    "@mamacord//api.star",
    "embed",
    "modal",
    "modal_field",
    "reply",
    "update",
)
load(
    "//lib:helpers.star",
    "counter_effect",
    "guild_error",
    "parse_int",
    "render_counter",
)


def modal_handler(ctx):
    error = guild_error(ctx, update_message=True)
    if error != None:
        return [error]
    raw = ctx.modal_fields["value"]
    value = parse_int(raw)
    if value == None:
        return [reply(
            embeds=[embed(
                title=ctx.t("example.invalid.title"),
                description=ctx.t("example.invalid.body", {"Raw": raw}),
            )],
            ephemeral=True,
        )]
    return [
        counter_effect(ctx, value),
        render_counter(ctx, value, update_message=True),
    ]


SET_COUNTER_MODAL = modal(
    id="set_counter",
    handler=modal_handler,
    fields=[modal_field(id="value", required=True)],
)
