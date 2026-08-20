load("@mamacord//api.star", "cog", "plugin")
load("//commands:about.star", "ABOUT")
load("//commands:lookup.star", "LOOKUP")


def setup(bot):
    bot.add_cog(cog(
        name="Info",
        commands=[ABOUT, LOOKUP],
    ))


PLUGIN = plugin(setup=setup)
