package contract

import (
	"strings"
	"testing"
)

func TestDefinitionValidationAndDeepClone(t *testing.T) {
	t.Parallel()

	definition := validDefinition()
	if err := definition.Validate(); err != nil {
		t.Fatalf("valid definition rejected: %v", err)
	}
	definition.Cogs[0].Commands = append(definition.Cogs[0].Commands, CommandDefinition{Kind: CommandUser, Name: "tool", Route: "command:user:tool"})
	if err := definition.Validate(); err != nil {
		t.Fatalf("same name in another command kind rejected: %v", err)
	}
	clone := definition.DeepClone()
	clone.Cogs[0].Commands[0].Children[0].Options[0].Name = "changed"
	clone.Cogs[0].Components[0].Checks[0].Permissions[0] = "changed"
	if definition.Cogs[0].Commands[0].Children[0].Options[0].Name != "text" {
		t.Fatal("command option was aliased")
	}
	if definition.Cogs[0].Components[0].Checks[0].Permissions[0] != "manage_messages" {
		t.Fatal("check permissions were aliased")
	}
}

func TestDefinitionRejectsInvalidShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mutate   func(*Definition)
		contains string
	}{
		{name: "duplicate route", mutate: func(d *Definition) { d.Cogs[0].Components[0].Route = "command:tool:create" }, contains: "duplicate route"},
		{name: "required after optional", mutate: func(d *Definition) {
			options := d.Cogs[0].Commands[0].Children[0].Options
			options[0].Required = false
			options[1].Required = true
		}, contains: "follows an optional"},
		{name: "nested group", mutate: func(d *Definition) {
			d.Cogs[0].Commands[0].Children[0].Route = ""
			d.Cogs[0].Commands[0].Children[0].Options = nil
			d.Cogs[0].Commands[0].Children[0].Children = []CommandDefinition{{Kind: CommandGroup, Name: "nested", CommandDescription: CommandDescription{Description: "nested"}, Children: []CommandDefinition{{Kind: CommandSubcommand, Name: "leaf", CommandDescription: CommandDescription{Description: "leaf"}, Route: "nested"}}}}
		}, contains: "cannot contain children"},
		{name: "container route", mutate: func(d *Definition) { d.Cogs[0].Commands[0].Route = "container" }, contains: "cannot have a route"},
		{name: "duplicate component", mutate: func(d *Definition) { d.Cogs[0].Components = append(d.Cogs[0].Components, d.Cogs[0].Components[0]) }, contains: "duplicate component"},
		{name: "choice kind", mutate: func(d *Definition) {
			d.Cogs[0].Commands[0].Children[0].Options[0].Choices = []ChoiceDefinition{{Name: "one", Value: ChoiceValue{Kind: ChoiceInteger, Integer: 1}}}
		}, contains: "kind does not match"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			definition := validDefinition()
			test.mutate(&definition)
			err := definition.Validate()
			if err == nil || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("expected %q error, got %v", test.contains, err)
			}
		})
	}
}

func validDefinition() Definition {
	minLength, maxLength := 1, 100
	minimum, maximum := int64(1), int64(10)
	return Definition{Cogs: []CogDefinition{{
		Name:   "Tools",
		Checks: []CheckDefinition{{Kind: CheckGuildOnly}},
		Commands: []CommandDefinition{{
			Kind: CommandSlash,
			Name: "tool", CommandDescription: CommandDescription{Description: "Tool commands"}, Children: []CommandDefinition{{
				Kind:  CommandSubcommand,
				Route: "command:tool:create",
				Name:  "create", CommandDescription: CommandDescription{Description: "Create a tool"}, Options: []OptionDefinition{
					{Name: "text", Kind: OptionString, OptionDescription: OptionDescription{Description: "Text"}, Required: true, StringOptionBounds: StringOptionBounds{MinLength: &minLength, MaxLength: &maxLength}},
					{Name: "count", Kind: OptionInteger, OptionDescription: OptionDescription{Description: "Count"}, IntegerOptionBounds: IntegerOptionBounds{MinInteger: &minimum, MaxInteger: &maximum}, OptionSelection: OptionSelection{Autocomplete: "autocomplete:tool:count"}},
				},
			}},
		}},
		Listeners:  []ListenerDefinition{{ID: "member_join", Event: "guild_member_join", Route: "listener:member_join"}},
		Tasks:      []TaskDefinition{{ID: "sweep", Schedule: "*/5 * * * *", Route: "task:sweep"}},
		Components: []ComponentDefinition{{ID: "save", Route: "component:save", Kinds: []ComponentKind{ComponentButton}, Checks: []CheckDefinition{{Kind: CheckHasPermissions, Permissions: []MemberPermission{PermissionManageMessages}}}}},
		Modals:     []ModalDefinition{{ID: "edit", Route: "modal:edit", Fields: []ModalFieldDefinition{{ID: "text", Required: true}}}},
	}}}
}
