package commands

type ModuleDescriptor struct {
	ID             string
	Name           string
	DefaultEnabled bool
	Toggleable     bool
	Definitions    func() []SlashCommand
}

func Catalog() []ModuleDescriptor {
	return []ModuleDescriptor{
		{
			ID:             "core",
			Name:           "Core",
			DefaultEnabled: true,
			Toggleable:     false,
			Definitions:    coreDefinitions,
		},
		{
			ID:             "admin",
			Name:           "Admin",
			DefaultEnabled: true,
			Toggleable:     false,
			Definitions:    adminDefinitions,
		},
	}
}
