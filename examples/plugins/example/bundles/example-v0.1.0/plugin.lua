local counter = bot.require("lib/counter.lua")
local ui = bot.ui
local i18n = bot.i18n

local function must_be_in_guild(ctx, message_type)
  if ctx.guild.id ~= "" then
    return nil
  end

  if message_type == "update" then
    return ui.update({ content = "This must be used in a server." })
  end

  return ui.present({
    kind = "error",
    title = i18n.t("example.not_in_guild.title", nil, nil),
    body = i18n.t("example.not_in_guild.body", nil, nil),
    ephemeral = true
  })
end

local function render_counter(count, message_type)
  local response = {
    ephemeral = true,
    content = i18n.t("example.counter", { Count = count }, nil),
    components = {
      {
        ui.button("inc", { label = "Increment", style = "primary" }),
        ui.button("set", { label = "Set...", style = "secondary" })
      }
    }
  }

  if message_type == "update" then
    return ui.update(response)
  end
  return ui.reply(response)
end

return bot.plugin({
  commands = {
    bot.command("example", {
      description = "Example Lua plugin command",
      description_id = "cmd.example.desc",
      ephemeral = true,
      run = function(ctx)
        bot.log.info("command " .. ctx.command.name)

        local guild_error = must_be_in_guild(ctx, "message")
        if guild_error ~= nil then
          return guild_error
        end

        local count = counter.increment(ctx.store)
        return render_counter(count, "message")
      end
    })
  },

  components = {
    inc = function(ctx)
      local guild_error = must_be_in_guild(ctx, "update")
      if guild_error ~= nil then
        return guild_error
      end

      local count = counter.increment(ctx.store)
      return render_counter(count, "update")
    end,

    set = function(ctx)
      local guild_error = must_be_in_guild(ctx, "update")
      if guild_error ~= nil then
        return guild_error
      end

      return ui.modal("set_counter", {
        title = i18n.t("example.set.title", nil, nil),
        components = {
          ui.text_input("value", {
            label = i18n.t("example.set.label", nil, nil),
            style = "short",
            required = true,
            placeholder = "123"
          })
        }
      })
    end
  },

  modals = {
    set_counter = function(ctx)
      local guild_error = must_be_in_guild(ctx, "message")
      if guild_error ~= nil then
        return guild_error
      end

      local fields = ctx.modal.fields or {}
      local raw = fields.value
      local count = tonumber(raw)
      if count == nil then
        return ui.present({
          kind = "error",
          title = i18n.t("example.invalid.title", nil, nil),
          body = i18n.t("example.invalid.body", { Raw = tostring(raw) }, nil),
          ephemeral = true
        })
      end

      counter.set(ctx.store, count)
      return render_counter(count, "update")
    end
  }
})
