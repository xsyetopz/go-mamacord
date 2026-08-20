load("@mamacord//api.star", "cog", "plugin")
load("//commands:8ball.star", "EIGHT_BALL")
load("//commands:flip.star", "FLIP")
load("//commands:kawaii.star", "HUG", "PAT", "POKE", "SHRUG")
load("//commands:roll.star", "ROLL")


def setup(bot):
    bot.add_cog(cog(
        name="Fun",
        commands=[FLIP, ROLL, EIGHT_BALL, HUG, PAT, POKE, SHRUG],
    ))


PLUGIN = plugin(setup=setup)
