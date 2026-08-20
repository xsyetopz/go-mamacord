package commands

func adminDefinitions() []SlashCommand {
	return []SlashCommand{
		blockDefinition(),
		modulesDefinition(),
		pluginsDefinition(),
		unblockDefinition(),
	}
}

func blockDefinition() SlashCommand {
	return SlashCommand{
		Name:   "block",
		NameID: "cmd.block.name",
		DescID: "cmd.block.desc",
	}
}

func modulesDefinition() SlashCommand {
	return SlashCommand{
		Name: "modules",
	}
}

func pluginsDefinition() SlashCommand {
	return SlashCommand{
		Name:   "plugins",
		NameID: "cmd.plugins.name",
		DescID: "cmd.plugins.desc",
	}
}

func unblockDefinition() SlashCommand {
	return SlashCommand{
		Name:   "unblock",
		NameID: "cmd.unblock.name",
		DescID: "cmd.unblock.desc",
	}
}
