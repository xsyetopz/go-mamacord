load(
    "@mamacord//api.star",
    "button",
    "component",
    "show_modal",
    "modal_view",
    "text_input",
)
load(
    "//lib:helpers.star",
    "counter",
    "counter_effect",
    "guild_error",
    "render_counter",
)


def increment_handler(ctx):
    error = guild_error(ctx, update_message=True)
    if error != None:
        return [error]
    value = counter(ctx) + 1
    return [
        counter_effect(ctx, value),
        render_counter(ctx, value, update_message=True),
    ]


def set_handler(ctx):
    error = guild_error(ctx, update_message=True)
    if error != None:
        return [error]
    return [show_modal(view=modal_view(
        handler="set_counter",
        title=ctx.t("example.set.title"),
        fields=[text_input(
            id="value",
            label=ctx.t("example.set.label"),
            style="short",
            required=True,
            placeholder="123",
            min_length=1,
            max_length=20,
        )],
    ))]


INCREMENT_COMPONENT = component(
    id="inc",
    handler=increment_handler,
    kinds=["button"],
    defer="update",
)

SET_COMPONENT = component(
    id="set",
    handler=set_handler,
    kinds=["button"],
)
