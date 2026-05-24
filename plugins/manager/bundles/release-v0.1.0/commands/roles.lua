local option = bot.option
local i18n = bot.i18n
local ui = bot.ui
local shared = bot.require("lib/shared.lua")

local function create_subcommand()
  return {
    name = "create",
    description = "Create a new role.",
    description_id = "cmd.roles.sub.create.desc",
    options = {
      option.string("name", {
        description = "Role name.",
        description_id = "cmd.roles.opt.name.desc",
        required = true,
        min_length = 1,
        max_length = 100,
      }),
      option.string("colour", {
        description = "Hex role colour, like #57F287.",
        description_id = "cmd.roles.opt.colour.desc",
        min_length = 6,
        max_length = 7,
      }),
      option.bool("hoist", {
        description = "Show the role separately.",
        description_id = "cmd.roles.opt.hoist.desc",
      }),
      option.bool("mentionable", {
        description = "Allow mentioning the role.",
        description_id = "cmd.roles.opt.mentionable.desc",
      }),
    },
  }
end

local function edit_subcommand()
  return {
    name = "edit",
    description = "Edit an existing role.",
    description_id = "cmd.roles.sub.edit.desc",
    options = {
      option.role("role", {
        description = "Role to edit.",
        description_id = "cmd.roles.opt.role.desc",
        required = true,
      }),
      option.string("name", {
        description = "New role name.",
        description_id = "cmd.roles.opt.name.desc",
        min_length = 1,
        max_length = 100,
      }),
      option.string("colour", {
        description = "Hex role colour, like #57F287.",
        description_id = "cmd.roles.opt.colour.desc",
        min_length = 6,
        max_length = 7,
      }),
      option.bool("hoist", {
        description = "Show the role separately.",
        description_id = "cmd.roles.opt.hoist.desc",
      }),
      option.bool("mentionable", {
        description = "Allow mentioning the role.",
        description_id = "cmd.roles.opt.mentionable.desc",
      }),
    },
  }
end

local function delete_subcommand()
  return {
    name = "delete",
    description = "Delete a role.",
    description_id = "cmd.roles.sub.delete.desc",
    options = {
      option.role("role", {
        description = "Role to delete.",
        description_id = "cmd.roles.opt.role.desc",
        required = true,
      }),
    },
  }
end

local function add_subcommand()
  return {
    name = "add",
    description = "Add a role to a member.",
    description_id = "cmd.roles.sub.add.desc",
    options = {
      option.role("role", {
        description = "Role to add.",
        description_id = "cmd.roles.opt.role.desc",
        required = true,
      }),
      option.user("member", {
        description = "Member to update.",
        description_id = "cmd.roles.opt.member.desc",
        required = true,
      }),
    },
  }
end

local function remove_subcommand()
  return {
    name = "remove",
    description = "Remove a role from a member.",
    description_id = "cmd.roles.sub.remove.desc",
    options = {
      option.role("role", {
        description = "Role to remove.",
        description_id = "cmd.roles.opt.role.desc",
        required = true,
      }),
      option.user("member", {
        description = "Member to update.",
        description_id = "cmd.roles.opt.member.desc",
        required = true,
      }),
    },
  }
end

local function parsed_colour(ctx)
  local raw = shared.trim(ctx.command.args.colour)
  if raw == "" then
    return nil, true
  end
  local parsed = shared.parse_hex_color(raw)
  if parsed == nil then
    return nil, false
  end
  return parsed, true
end

local function role_ref(ctx)
  local role_id = shared.trim(ctx.command.args.role)
  return role_id, shared.resolved(ctx, "role")
end

return bot.command("roles", {
  description = "Manage roles.",
  description_id = "cmd.roles.desc",
  ephemeral = true,
  default_member_permissions = { "manage_roles" },
  subcommands = {
    create_subcommand(),
    edit_subcommand(),
    delete_subcommand(),
    add_subcommand(),
    remove_subcommand(),
  },
  run = function(ctx)
    local guild_error = shared.ensure_guild(ctx, i18n)
    if guild_error ~= nil then
      return guild_error
    end

    local subcommand = shared.trim(ctx.command.subcommand)
    if subcommand == "create" then
      local color, ok = parsed_colour(ctx)
      if not ok then
        return shared.error(i18n.t("mgr.roles.invalid_colour", {
          Colour = shared.trim(ctx.command.args.colour),
        }, nil))
      end

      local role, err = bot.discord.create_role({
        guild_id = ctx.guild.id,
        name = shared.trim(ctx.command.args.name),
        color = color,
        hoist = ctx.command.args.hoist,
        mentionable = ctx.command.args.mentionable,
      })
      if role == nil then
        return shared.error(i18n.t("mgr.roles.create_error", {
          Name = shared.trim(ctx.command.args.name),
        }, nil))
      end
      return shared.success(i18n.t("mgr.roles.create_success", {
        Role = role.mention,
        Name = shared.trim(ctx.command.args.name),
      }, nil))
    end

    if subcommand == "edit" then
      local role_id, resolved_role = role_ref(ctx)
      if role_id == ctx.guild.id then
        return shared.error(i18n.t("mgr.roles.cannot_edit_everyone", nil, nil))
      end

      local color, ok = parsed_colour(ctx)
      if not ok then
        return shared.error(i18n.t("mgr.roles.invalid_colour", {
          Colour = shared.trim(ctx.command.args.colour),
        }, nil))
      end

      local role, err = bot.discord.edit_role({
        guild_id = ctx.guild.id,
        role_id = role_id,
        name = shared.trim(ctx.command.args.name),
        color = color,
        hoist = ctx.command.args.hoist,
        mentionable = ctx.command.args.mentionable,
      })
      if role == nil then
        return shared.error(i18n.t("mgr.roles.edit_error", {
          Role = shared.mention_role(role_id),
        }, nil))
      end
      return shared.success(i18n.t("mgr.roles.edit_success", {
        Role = resolved_role.mention or shared.mention_role(role_id),
      }, nil))
    end

    if subcommand == "delete" then
      local role_id, resolved_role = role_ref(ctx)
      if role_id == ctx.guild.id then
        return shared.error(i18n.t("mgr.roles.cannot_delete_everyone", nil, nil))
      end

      local ok, err = bot.discord.delete_role({
        guild_id = ctx.guild.id,
        role_id = role_id,
      })
      if not ok then
        return shared.error(i18n.t("mgr.roles.delete_error", {
          Role = resolved_role.mention or shared.mention_role(role_id),
        }, nil))
      end
      return shared.success(i18n.t("mgr.roles.delete_success", {
        Name = resolved_role.name or shared.mention_role(role_id),
      }, nil))
    end

    if subcommand == "add" or subcommand == "remove" then
      local role_id, resolved_role = role_ref(ctx)
      local member_id = shared.trim(ctx.command.args.member)
      local member = shared.resolved(ctx, "member")
      local ok, err
      if subcommand == "add" then
        ok, err = bot.discord.add_role({
          guild_id = ctx.guild.id,
          user_id = member_id,
          role_id = role_id,
        })
      else
        ok, err = bot.discord.remove_role({
          guild_id = ctx.guild.id,
          user_id = member_id,
          role_id = role_id,
        })
      end
      if not ok then
        local message_id = "mgr.roles.remove_error"
        if subcommand == "add" then
          message_id = "mgr.roles.add_error"
        end
        return shared.error(i18n.t(message_id, {
          Role = resolved_role.mention or shared.mention_role(role_id),
          User = member.mention or shared.mention_user(member_id),
        }, nil))
      end

      local message_id = "mgr.roles.remove_success"
      if subcommand == "add" then
        message_id = "mgr.roles.add_success"
      end
      return shared.success(i18n.t(message_id, {
        Role = resolved_role.mention or shared.mention_role(role_id),
        User = member.mention or shared.mention_user(member_id),
      }, nil))
    end

    return ui.reply({
      ephemeral = true,
      content = i18n.t("err.generic", nil, nil),
    })
  end
})
