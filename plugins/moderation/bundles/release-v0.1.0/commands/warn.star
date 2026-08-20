load(
    "@mamacord//api.star",
    "append_audit",
    "attempt",
    "best_effort",
    "create_warning",
    "reply",
    "send_dm",
    "slash_command",
    "string_option",
    "timeout_member",
    "user_option",
)
load(
    "//lib:helpers.star",
    "command_error",
    "config",
    "guild_error",
    "mention",
)


def warn(ctx):
    error = guild_error(ctx)
    if error != None:
        return [error]

    target = ctx.option("user")
    reason = ctx.option("reason").strip()
    settings = config(ctx)
    if not settings["enabled"]:
        return [command_error(ctx)]
    if target["id"] == ctx.author["id"]:
        return [reply(content=ctx.t("mod.warn.self"), ephemeral=True)]
    if target["bot"] or target["system"]:
        return [reply(content=ctx.t("mod.warn.bot"), ephemeral=True)]

    count = ctx.count_warnings(target["id"])
    if count >= settings["warning_limit"]:
        return [reply(
            content=ctx.t("mod.warn.too_many", {"User": mention(target["id"])}),
            ephemeral=True,
        )]

    now = ctx.now_unix
    failure = reply(content=ctx.t("err.generic"), ephemeral=True)
    operations = [
        attempt(
            effect=create_warning(
                user_id=target["id"],
                reason=reason,
                created_at=now,
            ),
            on_error=failure,
        ),
        attempt(
            effect=append_audit(
                action="warn.create",
                target_type="user",
                target_id=target["id"],
                created_at=now,
                metadata={},
            ),
            on_error=failure,
        ),
    ]

    timeout_minutes = 0
    if count + 1 >= settings["timeout_threshold"]:
        timeout_minutes = settings["timeout_minutes"]
        until = now + timeout_minutes * 60
        timeout_failure = reply(
            content=ctx.t("mod.warn.success", {
                "User": mention(target["id"]),
                "Reason": reason,
                "TimeoutMinutes": 0,
                "TimeoutFailed": True,
            }),
            ephemeral=True,
        )
        operations.append(attempt(
            effect=timeout_member(user_id=target["id"], until_unix=until),
            on_error=timeout_failure,
        ))
        operations.append(best_effort(effect=append_audit(
            action="warn.timeout",
            target_type="user",
            target_id=target["id"],
            created_at=now,
            metadata={"until": until},
        )))

    operations.append(best_effort(effect=send_dm(
        user_id=target["id"],
        content=ctx.t("mod.warn.dm", {
            "Reason": reason,
            "TimeoutMinutes": timeout_minutes,
        }),
    )))
    operations.append(reply(
        content=ctx.t("mod.warn.success", {
            "User": mention(target["id"]),
            "Reason": reason,
            "TimeoutMinutes": timeout_minutes,
            "TimeoutFailed": False,
        }),
        ephemeral=True,
    ))
    return operations


WARN_COMMAND = slash_command(
    name="warn",
    description="Warn a member.",
    description_id="cmd.warn.desc",
    handler=warn,
    defer="create",
    ephemeral=True,
    permissions=["moderate_members"],
    options=[
        user_option(
            name="user",
            description="User to warn.",
            description_id="cmd.warn.opt.user.desc",
            required=True,
        ),
        string_option(
            name="reason",
            description="Reason.",
            description_id="cmd.warn.opt.reason.desc",
            required=True,
            min_length=1,
            max_length=255,
        ),
    ],
)
