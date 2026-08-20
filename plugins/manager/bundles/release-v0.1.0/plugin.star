load("@mamacord//api.star", "cog", "plugin")
load("//commands:channel.star", "CHANNEL_COMMANDS", "PURGE_COMMANDS")
load("//commands:emoji.star", "EMOJI_COMMANDS")
load("//commands:nickname.star", "NICKNAME_COMMANDS")
load("//commands:roles.star", "ROLE_COMMANDS")
load("//commands:sticker.star", "STICKER_COMMANDS")


def setup(bot):
    commands = (
        CHANNEL_COMMANDS
        + NICKNAME_COMMANDS
        + ROLE_COMMANDS
        + PURGE_COMMANDS
        + EMOJI_COMMANDS
        + STICKER_COMMANDS
    )
    bot.add_cog(cog(
        name="Manager",
        commands=commands,
    ))


PLUGIN = plugin(setup=setup)
