load("@mamacord//api.star", "slash_command", "string_option")
load("//lib:shared.star", "EIGHT_BALL_ANSWERS", "ERROR_COLOR", "embed_reply", "mention")


def eight_ball_handler(ctx):
    question = ctx.option("question")
    if len(question) < 3 or question[-1] not in "?.!":
        return [embed_reply(
            description=ctx.t(
                "fun.8ball.question_error",
                {"Question": question},
            ),
            color=ERROR_COLOR,
            ephemeral=True,
        )]
    answer = ctx.random_choice(EIGHT_BALL_ANSWERS)
    return [embed_reply(
        description=ctx.t(
            "fun.8ball.result",
            {"Answer": answer, "User": mention(ctx.author["id"])},
        ),
    )]


EIGHT_BALL = slash_command(
    name="8ball",
    description="Ask the Magic 8 Ball a question.",
    description_id="cmd.8ball.desc",
    handler=eight_ball_handler,
    options=[
        string_option(
            name="question",
            description="What do you want to ask?",
            description_id="cmd.8ball.opt.question.desc",
            required=True,
            min_length=3,
            max_length=255,
        ),
    ],
)
