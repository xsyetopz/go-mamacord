package contract

import (
	"encoding/json"
	"testing"
)

func TestComposedContractsKeepFlatJSONKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value any
		keys  []string
	}{
		{name: "route signature", value: RouteSignature{}, keys: []string{"Type", "Route", "CommandKind", "Defer", "Path", "Options", "OptionName", "OptionKind", "ComponentID", "ComponentKinds", "ModalID", "ModalFields", "Event", "TaskID"}},
		{name: "command definition", value: CommandDefinition{}, keys: []string{"Kind", "Route", "Name", "Description", "DescriptionID", "Ephemeral", "Defer", "DefaultMemberPermissions", "Options", "Children", "Checks"}},
		{name: "option definition", value: OptionDefinition{}, keys: []string{"Name", "Kind", "Description", "DescriptionID", "Required", "Choices", "MinInteger", "MaxInteger", "MinNumber", "MaxNumber", "MinLength", "MaxLength", "ChannelKinds", "Autocomplete"}},
		{name: "role reference", value: RoleRef{}, keys: []string{"ID", "GuildID", "Name", "Position", "Permissions", "Mention", "Color", "Hoist", "Mentionable", "Managed", "PermissionBits", "CreatedAt"}},
		{name: "option value", value: OptionValue{}, keys: []string{"Name", "Kind", "String", "Boolean", "Integer", "Number", "User", "Channel", "Role", "Mentionable", "Attachment"}},
		{name: "invocation", value: Invocation{}, keys: []string{"PluginID", "Generation", "Route", "Kind", "Guild", "Channel", "Author", "BotUser", "Member", "Locale", "NowUnix", "RandomSeed", "Runtime", "State", "IsOwner", "ResponseState", "ModalOrigin", "Command", "Autocomplete", "Component", "Modal", "Event", "Task", "Check"}},
		{name: "guild details", value: GuildDetailsRef{}, keys: []string{"Guild", "OwnerID", "Description", "IconURL", "BannerURL", "RolesCount", "EmojisCount", "StickersCount", "MemberCount", "ChannelsCount", "CreatedAt"}},
		{name: "reminder", value: ReminderRef{}, keys: []string{"ID", "Schedule", "Kind", "Note", "Delivery", "GuildID", "ChannelID", "Enabled", "NextRunAt", "LastRunAt", "FailureCount", "CreatedAt", "UpdatedAt"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := json.Marshal(test.value)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var object map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &object); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if len(object) != len(test.keys) {
				t.Fatalf("JSON keys = %v, want %v", object, test.keys)
			}
			for _, key := range test.keys {
				if _, ok := object[key]; !ok {
					t.Errorf("JSON is missing flat key %q: %s", key, encoded)
				}
			}
		})
	}
}
