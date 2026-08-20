load("@mamacord//api.star", "integer_option", "slash_command")
load("//lib:shared.star", "DICE_SIDES", "WARN_COLOR", "embed_reply", "mention")


def roll_handler(ctx):
    number = ctx.option("number")
    sides = ctx.option("sides")
    modifier_value = ctx.option("modifier")
    modifier = 0 if modifier_value == None else modifier_value
    if sides not in DICE_SIDES:
        return [embed_reply(
            description=ctx.t(
                "fun.roll.invalid_sides",
                {"Sides": sides},
            ),
            color=WARN_COLOR,
            ephemeral=True,
        )]
    rolls = []
    total = modifier
    for _ in range(number):
        value = ctx.random_int(1, sides)
        rolls.append(value)
        total += value
    notation = str(number) + "d" + str(sides)
    if modifier > 0:
        notation += "+" + str(modifier)
    elif modifier < 0:
        notation += str(modifier)
    verbose = ", ".join([str(value) for value in rolls])
    if modifier > 0:
        verbose += " + " + str(modifier)
    elif modifier < 0:
        verbose += " - " + str(-modifier)
    return [embed_reply(
        description=ctx.t(
            "fun.roll.result",
            {
                "User": mention(ctx.author["id"]),
                "Notation": notation,
                "Total": total,
            },
        ),
        footer=verbose,
    )]


ROLL = slash_command(
    name="roll",
    description="Roll some dice.",
    description_id="cmd.roll.desc",
    handler=roll_handler,
    options=[
        integer_option(
            name="number",
            description="How many dice?",
            description_id="cmd.roll.opt.number.desc",
            required=True,
            min_integer=1,
            max_integer=99,
        ),
        integer_option(
            name="sides",
            description="How many sides per die?",
            description_id="cmd.roll.opt.sides.desc",
            required=True,
            min_integer=4,
            max_integer=20,
        ),
        integer_option(
            name="modifier",
            description="Any modifier to add?",
            description_id="cmd.roll.opt.modifier.desc",
            min_integer=-99,
            max_integer=99,
        ),
    ],
)
