load("@mamacord//api.star", "group")
load("//commands:lookup_channel.star", "LOOKUP_CHANNEL")
load("//commands:lookup_guild.star", "LOOKUP_GUILD")
load("//commands:lookup_role.star", "LOOKUP_ROLE")
load("//commands:lookup_user.star", "LOOKUP_USER")


LOOKUP = group(
    name="lookup",
    description="Look up Discord objects.",
    description_id="cmd.lookup.desc",
    children=[
        LOOKUP_USER,
        LOOKUP_GUILD,
        LOOKUP_ROLE,
        LOOKUP_CHANNEL,
    ],
)
