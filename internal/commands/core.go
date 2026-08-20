package commands

func coreDefinitions() []SlashCommand {
	return []SlashCommand{
		ping(),
		help(),
	}
}

func ping() SlashCommand {
	return SlashCommand{
		Name:   "ping",
		NameID: "cmd.ping.name",
		DescID: "cmd.ping.desc",
	}
}

func help() SlashCommand {
	return SlashCommand{
		Name:   "help",
		NameID: "cmd.help.name",
		DescID: "cmd.help.desc",
	}
}
