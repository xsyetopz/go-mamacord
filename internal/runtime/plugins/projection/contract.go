package projection

import (
	"fmt"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
)

func CommandsFromContract(definition contract.Definition) ([]Command, error) {
	var commands []Command
	for _, cog := range definition.Cogs {
		for _, source := range cog.Commands {
			command, err := commandFromContract(source)
			if err != nil {
				return nil, fmt.Errorf("cog %q: %w", cog.Name, err)
			}
			commands = append(commands, command)
		}
	}
	return commands, nil
}
func commandFromContract(source contract.CommandDefinition) (Command, error) {
	command := Command{Name: source.Name, Description: source.Description, DescriptionID: source.DescriptionID, Ephemeral: source.Ephemeral, Defer: source.Defer, DefaultMemberPermissions: permissionStrings(source.DefaultMemberPermissions)}
	switch source.Kind {
	case contract.CommandSlash:
		command.Type = CommandTypeSlash
		command.Options = optionsFromContract(source.Options)
	case contract.CommandUser:
		command.Type = CommandTypeUser
	case contract.CommandMessage:
		command.Type = CommandTypeMessage
	case contract.CommandGroup:
		command.Type = CommandTypeSlash
		for _, child := range source.Children {
			switch child.Kind {
			case contract.CommandSubcommand:
				command.Subcommands = append(command.Subcommands, subcommandFromContract(child))
			case contract.CommandGroup:
				group, err := groupFromContract(child)
				if err != nil {
					return Command{}, err
				}
				command.Groups = append(command.Groups, group)
			default:
				return Command{}, fmt.Errorf("top-level group %q has child kind %q", source.Name, child.Kind)
			}
		}
	default:
		return Command{}, fmt.Errorf("unsupported top-level command kind %q", source.Kind)
	}
	return command, nil
}
func subcommandFromContract(source contract.CommandDefinition) Subcommand {
	ephemeral := source.Ephemeral
	return Subcommand{Name: source.Name, Description: source.Description, DescriptionID: source.DescriptionID, Ephemeral: &ephemeral, Defer: source.Defer, Options: optionsFromContract(source.Options)}
}
func groupFromContract(source contract.CommandDefinition) (CommandGroup, error) {
	group := CommandGroup{Name: source.Name, Description: source.Description, DescriptionID: source.DescriptionID}
	for _, child := range source.Children {
		if child.Kind != contract.CommandSubcommand {
			return CommandGroup{}, fmt.Errorf("command group %q child %q is not a subcommand", source.Name, child.Name)
		}
		group.Subcommands = append(group.Subcommands, subcommandFromContract(child))
	}
	return group, nil
}
func optionsFromContract(sources []contract.OptionDefinition) []CommandOption {
	out := make([]CommandOption, len(sources))
	for index, source := range sources {
		option := CommandOption{
			OptionPresentation: OptionPresentation{
				Name: source.Name, Type: optionTypeString(source.Kind), Description: source.Description,
				DescriptionID: source.DescriptionID, Required: source.Required, Autocomplete: string(source.Autocomplete),
			},
			OptionBounds: OptionBounds{MinLength: cloneIntPointer(source.MinLength), MaxLength: cloneIntPointer(source.MaxLength)},
			ChannelTypes: channelTypeNumbers(source.ChannelKinds),
		}
		if source.MinInteger != nil {
			value := float64(*source.MinInteger)
			option.MinValue = &value
		}
		if source.MaxInteger != nil {
			value := float64(*source.MaxInteger)
			option.MaxValue = &value
		}
		if source.MinNumber != nil {
			value := *source.MinNumber
			option.MinValue = &value
		}
		if source.MaxNumber != nil {
			value := *source.MaxNumber
			option.MaxValue = &value
		}
		for _, choice := range source.Choices {
			option.Choices = append(option.Choices, OptionChoice{Name: choice.Name, Value: choiceValue(choice.Value)})
		}
		out[index] = option
	}
	return out
}
func optionTypeString(kind contract.OptionKind) string {
	switch kind {
	case contract.OptionBoolean:
		return "bool"
	case contract.OptionInteger:
		return "int"
	case contract.OptionNumber:
		return "float"
	default:
		return string(kind)
	}
}
func choiceValue(value contract.ChoiceValue) any {
	switch value.Kind {
	case contract.ChoiceString:
		return value.String
	case contract.ChoiceInteger:
		return value.Integer
	case contract.ChoiceNumber:
		return value.Number
	default:
		return nil
	}
}
func permissionStrings(values []contract.MemberPermission) []string {
	out := make([]string, len(values))
	for index, value := range values {
		out[index] = string(value)
	}
	return out
}
func cloneIntPointer(value *int) *int {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
func channelTypeNumbers(values []contract.ChannelKind) []int {
	out := make([]int, len(values))
	for index, value := range values {
		switch value {
		case contract.ChannelText:
			out[index] = 0
		case contract.ChannelVoice:
			out[index] = 2
		case contract.ChannelCategory:
			out[index] = 4
		case contract.ChannelAnnouncement:
			out[index] = 5
		case contract.ChannelStage:
			out[index] = 13
		case contract.ChannelForum:
			out[index] = 15
		case contract.ChannelMedia:
			out[index] = 16
		}
	}
	return out
}
func SubscriptionsFromContract(pluginID string, definition contract.Definition) ([]string, []Job) {
	var events []string
	var jobs []Job
	for _, cog := range definition.Cogs {
		for _, listener := range cog.Listeners {
			events = append(events, listener.Event)
		}
		for _, task := range cog.Tasks {
			jobs = append(jobs, Job{ID: task.ID, Schedule: task.Schedule})
		}
	}
	return events, jobs
}
