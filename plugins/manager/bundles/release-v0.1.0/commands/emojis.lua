local option = bot.option
local i18n = bot.i18n
local ui = bot.ui
local shared = bot.require("lib/shared.lua")

local function create_subcommand()
  return {
    name = "create",
    description = "Create a custom emoji.",
    description_id = "cmd.emojis.sub.create.desc",
    options = {
      option.string("name", {
        description = "Emoji name.",
        description_id = "cmd.emojis.opt.name.desc",
        required = true,
        min_length = 2,
        max_length = 32,
      }),
      option.attachment("file", {
        description = "Emoji image file.",
        description_id = "cmd.emojis.opt.file.desc",
        required = true,
      }),
    },
  }
end

local function edit_subcommand()
  return {
    name = "edit",
    description = "Rename a custom emoji.",
    description_id = "cmd.emojis.sub.edit.desc",
    options = {
      option.string("emoji", {
        description = "Emoji mention or ID.",
        description_id = "cmd.emojis.opt.emoji.desc",
        required = true,
        min_length = 1,
        max_length = 128,
      }),
      option.string("name", {
        description = "Emoji name.",
        description_id = "cmd.emojis.opt.name.desc",
        required = true,
        min_length = 2,
        max_length = 32,
      }),
    },
  }
end

local function delete_subcommand()
  return {
    name = "delete",
    description = "Delete a custom emoji.",
    description_id = "cmd.emojis.sub.delete.desc",
    options = {
      option.string("emoji", {
        description = "Emoji mention or ID.",
        description_id = "cmd.emojis.opt.emoji.desc",
        required = true,
        min_length = 1,
        max_length = 128,
      }),
    },
  }
end

local function create_error_response(file, err_text)
  local code, parts = shared.split_error(err_text)
  if code == "file_too_large" then
    return shared.error(i18n.t("mgr.emojis.file_too_large", {
      Max = shared.max_emoji_file_bytes,
      Size = file.size or 0,
    }, nil))
  end
  if code == "bad_extension" then
    return shared.error(i18n.t("mgr.emojis.bad_extension", {
      Ext = shared.attachment_extension(file),
    }, nil))
  end
  if code == "too_many" then
    return shared.error(i18n.t("mgr.emojis.too_many", {
      Max = tonumber(parts[1]) or 0,
    }, nil))
  end
  if code == "download_error" then
    return shared.error(i18n.t("mgr.emojis.download_error", nil, nil))
  end
  if code == "dimensions_error" then
    return shared.error(i18n.t("mgr.emojis.dimensions_error", nil, nil))
  end
  if code == "too_large_dims" then
    return shared.error(i18n.t("mgr.emojis.too_large_dims", {
      Width = tonumber(parts[1]) or file.width or 0,
      Height = tonumber(parts[2]) or file.height or 0,
    }, nil))
  end
  if code == "bad_image" then
    return shared.error(i18n.t("mgr.emojis.bad_image", nil, nil))
  end
  return shared.error(i18n.t("mgr.emojis.create_error", {
    Name = shared.trim(file.filename),
  }, nil))
end

return bot.command("emojis", {
  description = "Manage custom emojis.",
  description_id = "cmd.emojis.desc",
  ephemeral = true,
  default_member_permissions = { "manage_expressions", "create_expressions" },
  subcommands = {
    create_subcommand(),
    edit_subcommand(),
    delete_subcommand(),
  },
  run = function(ctx)
    local guild_error = shared.ensure_guild(ctx, i18n)
    if guild_error ~= nil then
      return guild_error
    end

    local subcommand = shared.trim(ctx.command.subcommand)
    if subcommand == "create" then
      local file = shared.attachment(ctx, "file")
      if file == nil then
        return shared.error(i18n.t("mgr.emojis.file_missing", nil, nil))
      end

      local result, err = bot.discord.create_emoji({
        guild_id = ctx.guild.id,
        name = shared.trim(ctx.command.args.name),
        file = file,
      })
      if result == nil then
        local code = shared.split_error(err)
        if code == "create_error" then
          return shared.error(i18n.t("mgr.emojis.create_error", {
            Name = shared.trim(ctx.command.args.name),
          }, nil))
        end
        return create_error_response(file, err)
      end

      return shared.success(i18n.t("mgr.emojis.create_success", {
        Name = shared.trim(ctx.command.args.name),
      }, nil))
    end

    if subcommand == "edit" then
      local result, err = bot.discord.edit_emoji({
        guild_id = ctx.guild.id,
        emoji = shared.trim(ctx.command.args.emoji),
        name = shared.trim(ctx.command.args.name),
      })
      if result == nil then
        local code = shared.split_error(err)
        if code == "invalid_emoji" then
          return shared.error(i18n.t("mgr.emojis.invalid_emoji", {
            Emoji = shared.trim(ctx.command.args.emoji),
          }, nil))
        end
        return shared.error(i18n.t("mgr.emojis.edit_error", nil, nil))
      end
      return shared.success(i18n.t("mgr.emojis.edit_success", {
        Name = shared.trim(ctx.command.args.name),
      }, nil))
    end

    if subcommand == "delete" then
      local ok, err = bot.discord.delete_emoji({
        guild_id = ctx.guild.id,
        emoji = shared.trim(ctx.command.args.emoji),
      })
      if not ok then
        local code = shared.split_error(err)
        if code == "invalid_emoji" then
          return shared.error(i18n.t("mgr.emojis.invalid_emoji", {
            Emoji = shared.trim(ctx.command.args.emoji),
          }, nil))
        end
        return shared.error(i18n.t("mgr.emojis.delete_error", nil, nil))
      end
      return shared.success(i18n.t("mgr.emojis.delete_success", nil, nil))
    end

    return ui.reply({
      ephemeral = true,
      content = i18n.t("err.generic", nil, nil),
    })
  end
})
