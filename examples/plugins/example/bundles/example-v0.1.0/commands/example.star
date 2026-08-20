load(
    "@mamacord//api.star",
    "reply",
    "slash_command",
)
load(
    "//lib:helpers.star",
    "counter",
    "counter_effect",
    "guild_error",
    "render_counter",
)


def command_handler(ctx):
    error = guild_error(ctx)
    if error != None:
        return [error]
    value = counter(ctx) + 1
    return [counter_effect(ctx, value), render_counter(ctx, value)]


EXAMPLE_COMMAND = slash_command(
    name="example",
    description="Example Starlark plugin command",
    description_id="cmd.example.desc",
    ephemeral=True,
    handler=command_handler,
)
