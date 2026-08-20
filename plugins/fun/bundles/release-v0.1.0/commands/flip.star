load("@mamacord//api.star", "slash_command")
load("//lib:shared.star", "embed_reply", "mention")


def flip_handler(ctx):
    result = "tails" if ctx.random_int(0, 1) == 0 else "heads"
    return [embed_reply(
        description=ctx.t(
            "fun.flip.result",
            {"User": mention(ctx.author["id"]), "Result": result},
        ),
    )]


FLIP = slash_command(
    name="flip",
    description="Flip a coin.",
    description_id="cmd.flip.desc",
    handler=flip_handler,
)
