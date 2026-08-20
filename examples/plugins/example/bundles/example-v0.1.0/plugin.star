load("@mamacord//api.star", "cog", "plugin")
load("//commands:example.star", "EXAMPLE_COMMAND")
load("//components:buttons.star", "INCREMENT_COMPONENT", "SET_COMPONENT")
load("//components:modal.star", "SET_COUNTER_MODAL")


def setup(bot):
    bot.add_cog(cog(
        name="Example",
        commands=[EXAMPLE_COMMAND],
        components=[INCREMENT_COMPONENT, SET_COMPONENT],
        modals=[SET_COUNTER_MODAL],
    ))


PLUGIN = plugin(setup=setup)
