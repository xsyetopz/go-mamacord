load(
    "@mamacord//api.star",
    "attempt",
    "component",
    "delete_reminder",
    "update",
)


def delete_component(ctx):
    if len(ctx.selected_values) != 1:
        return [update(content=ctx.t("err.generic"), components=[])]
    return [
        attempt(
            effect=delete_reminder(reminder_id=ctx.selected_values[0]),
            on_error=update(content=ctx.t("wellness.remind.delete.not_found"), components=[]),
        ),
        update(content=ctx.t("wellness.remind.delete.success"), components=[]),
    ]


DELETE_REMINDER_COMPONENT = component(
    id="delete_reminder",
    handler=delete_component,
    kinds=["string_select"],
    defer="update",
)
