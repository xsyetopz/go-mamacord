load("@mamacord//api.star", "embed", "embed_footer", "reply")

FUN_COLOR = 0x5865F2
WARN_COLOR = 0xFEE75C
ERROR_COLOR = 0xED4245
DICE_SIDES = [4, 6, 8, 10, 12, 20]
EIGHT_BALL_ANSWERS = [
    "It is certain",
    "It is decidedly so",
    "Without a doubt",
    "Yes - definitely",
    "You may rely on it",
    "As I see it, yes",
    "Most likely",
    "Outlook good",
    "Yes",
    "Signs point to yes",
    "Reply hazy, try again",
    "Ask again later",
    "Better not tell you now",
    "Cannot predict now",
    "Concentrate and ask again",
    "Don't count on it",
    "My reply is no",
    "My sources say no",
    "Outlook not so good",
    "Very doubtful",
]
ENDPOINT_EMOJI = {
    "hug": "🤗",
    "pat": "🫳",
    "poke": "👉",
    "shrug": "🤷",
}


def mention(user_id):
    return "<@" + user_id + ">"


def embed_reply(
    description="",
    color=FUN_COLOR,
    image_url="",
    footer="",
    content="",
    ephemeral=False,
):
    footer_value = None
    if footer:
        footer_value = embed_footer(text=footer)
    return reply(
        content=content,
        ephemeral=ephemeral,
        embeds=[embed(
            description=description,
            color=color,
            image_url=image_url,
            footer=footer_value,
        )],
    )
