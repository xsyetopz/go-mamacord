package core

import commandspec "github.com/xsyetopz/go-mamacord/internal/commands/spec"

func Definitions() []commandspec.SlashCommand {
	return []commandspec.SlashCommand{
		ping(),
		help(),
	}
}

func ping() commandspec.SlashCommand {
	return commandspec.SlashCommand{
		Name:   "ping",
		NameID: "cmd.ping.name",
		DescID: "cmd.ping.desc",
	}
}

func help() commandspec.SlashCommand {
	return commandspec.SlashCommand{
		Name:   "help",
		NameID: "cmd.help.name",
		DescID: "cmd.help.desc",
	}
}
