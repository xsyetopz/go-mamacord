load("@mamacord//api.star", "cog", "plugin")
load("//commands:checkin.star", "CHECKIN_COMMAND")
load("//commands:remind.star", "REMINDER_COMMAND")
load("//commands:timezone.star", "TIMEZONE_COMMAND")
load("//components:delete_reminder.star", "DELETE_REMINDER_COMPONENT")


def setup(bot):
    bot.add_cog(cog(
        name="Wellness",
        commands=[TIMEZONE_COMMAND, CHECKIN_COMMAND, REMINDER_COMMAND],
        components=[DELETE_REMINDER_COMPONENT],
    ))


PLUGIN = plugin(setup=setup)
