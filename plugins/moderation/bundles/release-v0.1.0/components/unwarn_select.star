load(
    "@mamacord//api.star",
    "append_audit",
    "attempt",
    "best_effort",
    "component",
    "delete_warning",
    "has_permissions",
    "update",
)
load("//lib:helpers.star", "UNWARN_TTL_SECONDS", "config", "mention", "parse_flow")


def unwarn_select(ctx):
    if ctx.guild == None or len(ctx.selected_values) != 1:
        return [update(content=ctx.t("err.generic"), components=[])]
    if not config(ctx)["enabled"]:
        return [update(content=ctx.t("err.generic"), components=[])]

    flow = parse_flow(ctx.selected_values[0])
    if flow == None:
        return [update(content=ctx.t("err.generic"), components=[])]
    if ctx.author["id"] != flow["actor_id"]:
        return [update(content=ctx.t("err.generic"), components=[])]
    if ctx.now_unix - flow["issued_at"] > UNWARN_TTL_SECONDS:
        return [update(content=ctx.t("mod.unwarn.expired"), components=[])]

    warnings = ctx.list_warnings(flow["target_id"], limit=100)
    found = False
    for warning in warnings:
        if warning["id"] == flow["warning_id"]:
            found = True
    if not found:
        return [update(content=ctx.t("err.generic"), components=[])]

    failure = update(content=ctx.t("err.generic"), components=[])
    return [
        attempt(
            effect=delete_warning(
                warning_id=flow["warning_id"],
                target_user_id=flow["target_id"],
            ),
            on_error=failure,
        ),
        best_effort(effect=append_audit(
            action="warn.delete",
            target_type="user",
            target_id=flow["target_id"],
            created_at=ctx.now_unix,
            metadata={},
        )),
        update(
            content=ctx.t("mod.unwarn.success", {"User": mention(flow["target_id"])}),
            components=[],
        ),
    ]


UNWARN_SELECT_COMPONENT = component(
    id="unwarn_select",
    handler=unwarn_select,
    kinds=["string_select"],
    defer="update",
    checks=[has_permissions(["moderate_members"])],
)
