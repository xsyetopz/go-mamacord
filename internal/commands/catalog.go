package commands

import (
	cmdadmin "github.com/xsyetopz/go-mamacord/internal/commands/admin"
	cmdcore "github.com/xsyetopz/go-mamacord/internal/commands/core"

	commandapi "github.com/xsyetopz/go-mamacord/internal/commands/api"
)

type ModuleDescriptor struct {
	ID             string
	Name           string
	DefaultEnabled bool
	Toggleable     bool
	Commands       func() []commandapi.SlashCommand
}

func Catalog() []ModuleDescriptor {
	return []ModuleDescriptor{
		{
			ID:             "core",
			Name:           "Core",
			DefaultEnabled: true,
			Toggleable:     false,
			Commands:       cmdcore.Commands,
		},
		{
			ID:             "admin",
			Name:           "Admin",
			DefaultEnabled: true,
			Toggleable:     false,
			Commands:       cmdadmin.Commands,
		},
	}
}

func All() []commandapi.SlashCommand {
	out := []commandapi.SlashCommand{}
	for _, module := range Catalog() {
		out = append(out, module.Commands()...)
	}
	return out
}
