local option = bot.option
local i18n = bot.i18n
local ui = bot.ui
local shared = bot.require("lib/shared.lua")

local function create_subcommand()
  return {
    name = "create",
    description = "Create a new sticker.",
    description_id = "cmd.stickers.sub.create.desc",
    options = {
      option.string("name", {
        description = "Sticker name.",
        description_id = "cmd.stickers.opt.name.desc",
        required = true,
        min_length = 2,
        max_length = 30,
      }),
      option.string("emoji_tag", {
        description = "Sticker emoji tag.",
        description_id = "cmd.stickers.opt.emoji_tag.desc",
        required = true,
        min_length = 1,
        max_length = 64,
      }),
      option.attachment("file", {
        description = "Sticker file.",
        description_id = "cmd.stickers.opt.file.desc",
        required = true,
      }),
      option.string("description", {
        description = "Sticker description.",
        description_id = "cmd.stickers.opt.description.desc",
        min_length = 2,
        max_length = 100,
      }),
    },
  }
end

local function edit_subcommand()
  return {
    name = "edit",
    description = "Edit a sticker.",
    description_id = "cmd.stickers.sub.edit.desc",
    options = {
      option.string("id", {
        description = "Sticker ID or link.",
        description_id = "cmd.stickers.opt.id.desc",
        required = true,
        min_length = 1,
        max_length = 255,
      }),
      option.string("name", {
        description = "Sticker name.",
        description_id = "cmd.stickers.opt.name.desc",
        required = true,
        min_length = 2,
        max_length = 30,
      }),
      option.string("description", {
        description = "Sticker description.",
        description_id = "cmd.stickers.opt.description.desc",
        min_length = 2,
        max_length = 100,
      }),
    },
  }
end

local function delete_subcommand()
  return {
    name = "delete",
    description = "Delete a sticker.",
    description_id = "cmd.stickers.sub.delete.desc",
    options = {
      option.string("id", {
        description = "Sticker ID or link.",
        description_id = "cmd.stickers.opt.id.desc",
        required = true,
        min_length = 1,
        max_length = 255,
      }),
    },
  }
end

local function create_error_response(file, name, err_text)
  local code, parts = shared.split_error(err_text)
  if code == "file_too_large" then
    return shared.error(i18n.t("mgr.stickers.file_too_large", {
      Max = shared.max_sticker_file_bytes,
      Size = file.size or 0,
    }, nil))
  end
  if code == "bad_extension" then
    return shared.error(i18n.t("mgr.stickers.bad_extension", {
      Ext = shared.attachment_extension(file),
    }, nil))
  end
  if code == "too_many" then
    return shared.error(i18n.t("mgr.stickers.too_many", {
      Max = tonumber(parts[1]) or 0,
    }, nil))
  end
  if code == "download_error" then
    return shared.error(i18n.t("mgr.stickers.download_error", nil, nil))
  end
  if code == "dimensions_error" then
    return shared.error(i18n.t("mgr.stickers.dimensions_error", nil, nil))
  end
  if code == "too_large_dims" then
    return shared.error(i18n.t("mgr.stickers.too_large_dims", {
      Width = tonumber(parts[1]) or file.width or 0,
      Height = tonumber(parts[2]) or file.height or 0,
    }, nil))
  end
  return shared.error(i18n.t("mgr.stickers.create_error", {
    Name = shared.trim(name),
  }, nil))
end

return bot.command("stickers", {
  description = "Manage server stickers.",
  description_id = "cmd.stickers.desc",
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
        return shared.error(i18n.t("mgr.stickers.file_missing", nil, nil))
      end

      local emoji_tag = shared.trim(ctx.command.args.emoji_tag)
      if emoji_tag == "" then
        return shared.error(i18n.t("mgr.stickers.tags_missing", nil, nil))
      end

      local result, err = bot.discord.create_sticker({
        guild_id = ctx.guild.id,
        name = shared.trim(ctx.command.args.name),
        description = shared.trim(ctx.command.args.description),
        emoji_tag = emoji_tag,
        file = file,
      })
      if result == nil then
        local code = shared.split_error(err)
        if code == "create_error" then
          return shared.error(i18n.t("mgr.stickers.create_error", {
            Name = shared.trim(ctx.command.args.name),
          }, nil))
        end
        return create_error_response(file, ctx.command.args.name, err)
      end

      return shared.success(i18n.t("mgr.stickers.create_success", {
        Name = shared.trim(ctx.command.args.name),
      }, nil))
    end

    if subcommand == "edit" then
      local result, err = bot.discord.edit_sticker({
        guild_id = ctx.guild.id,
        id = shared.trim(ctx.command.args.id),
        name = shared.trim(ctx.command.args.name),
        description = shared.trim(ctx.command.args.description),
      })
      if result == nil then
        local code = shared.split_error(err)
        if code == "invalid_id" then
          return shared.error(i18n.t("mgr.stickers.invalid_id", {
            ID = shared.trim(ctx.command.args.id),
          }, nil))
        end
        return shared.error(i18n.t("mgr.stickers.edit_error", nil, nil))
      end
      return shared.success(i18n.t("mgr.stickers.edit_success", {
        Name = shared.trim(ctx.command.args.name),
      }, nil))
    end

    if subcommand == "delete" then
      local ok, err = bot.discord.delete_sticker({
        guild_id = ctx.guild.id,
        id = shared.trim(ctx.command.args.id),
      })
      if not ok then
        local code = shared.split_error(err)
        if code == "invalid_id" then
          return shared.error(i18n.t("mgr.stickers.invalid_id", {
            ID = shared.trim(ctx.command.args.id),
          }, nil))
        end
        return shared.error(i18n.t("mgr.stickers.delete_error", nil, nil))
      end
      return shared.success(i18n.t("mgr.stickers.delete_success", nil, nil))
    end

    return ui.reply({
      ephemeral = true,
      content = i18n.t("err.generic", nil, nil),
    })
  end
})
