package admin

import commandspec "github.com/xsyetopz/go-mamacord/internal/commands/spec"

func Definitions() []commandspec.SlashCommand {
	return []commandspec.SlashCommand{
		blockDefinition(),
		modulesDefinition(),
		pluginsDefinition(),
		unblockDefinition(),
	}
}

func blockDefinition() commandspec.SlashCommand {
	return commandspec.SlashCommand{
		Name:   "block",
		NameID: "cmd.block.name",
		DescID: "cmd.block.desc",
	}
}

func modulesDefinition() commandspec.SlashCommand {
	return commandspec.SlashCommand{
		Name: "modules",
	}
}

func pluginsDefinition() commandspec.SlashCommand {
	return commandspec.SlashCommand{
		Name:   "plugins",
		NameID: "cmd.plugins.name",
		DescID: "cmd.plugins.desc",
	}
}

func unblockDefinition() commandspec.SlashCommand {
	return commandspec.SlashCommand{
		Name:   "unblock",
		NameID: "cmd.unblock.name",
		DescID: "cmd.unblock.desc",
	}
}
