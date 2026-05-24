package commands

import (
	"github.com/xsyetopz/go-mamacord/internal/commands/admin"
	"github.com/xsyetopz/go-mamacord/internal/commands/core"

	commandspec "github.com/xsyetopz/go-mamacord/internal/commands/spec"
)

type ModuleDescriptor struct {
	ID             string
	Name           string
	DefaultEnabled bool
	Toggleable     bool
	Definitions    func() []commandspec.SlashCommand
}

func Catalog() []ModuleDescriptor {
	return []ModuleDescriptor{
		{
			ID:             "core",
			Name:           "Core",
			DefaultEnabled: true,
			Toggleable:     false,
			Definitions:    core.Definitions,
		},
		{
			ID:             "admin",
			Name:           "Admin",
			DefaultEnabled: true,
			Toggleable:     false,
			Definitions:    admin.Definitions,
		},
	}
}
