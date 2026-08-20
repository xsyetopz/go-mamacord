load("@mamacord//api.star", "cog", "plugin")
load("//commands:unwarn.star", "UNWARN_COMMAND")
load("//commands:warn.star", "WARN_COMMAND")
load("//components:unwarn_select.star", "UNWARN_SELECT_COMPONENT")


def setup(bot):
    bot.add_cog(cog(
        name="Moderation",
        commands=[WARN_COMMAND, UNWARN_COMMAND],
        components=[UNWARN_SELECT_COMPONENT],
    ))


PLUGIN = plugin(setup=setup)
