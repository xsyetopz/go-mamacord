package starlark

import (
	"context"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
)

type LocalizationRequest struct {
	PluginID  string
	Locale    string
	MessageID string
	Data      contract.Value
}

type Localizer interface {
	Localize(context.Context, LocalizationRequest) (string, error)
}

type Reader interface {
	GetUser(context.Context, string) (contract.UserDetailsRef, bool, error)
	GetMember(context.Context, string, string) (contract.MemberDetailsRef, bool, error)
	GetGuild(context.Context, string) (contract.GuildDetailsRef, bool, error)
	NormalizeTimezone(context.Context, string) (string, bool, error)
	GetUserSettings(context.Context, string) (contract.UserSettingsRef, bool, error)
	ListCheckIns(context.Context, string, int) ([]contract.CheckInRef, error)
	PlanReminder(context.Context, string, string, int64) (contract.ReminderPlanRef, bool, error)
	ListReminders(context.Context, string, int) ([]contract.ReminderRef, error)
	CountWarnings(context.Context, string, string) (int, error)
	ListWarnings(context.Context, string, string, int) ([]contract.WarningRef, error)
}

type ResourceReader interface {
	ReadResource(context.Context, string) ([]byte, error)
}

type HTTPClient interface {
	GetJSON(context.Context, string, int64) (contract.Value, bool, error)
}

type InvocationServices struct {
	Localizer    Localizer
	Reader       Reader
	HTTP         HTTPClient
	Resources    ResourceReader
	HTTPHosts    []string
	Capabilities []contract.Capability
}

func (services InvocationServices) allows(capability contract.Capability) bool {
	for _, allowed := range services.Capabilities {
		if allowed == capability {
			return true
		}
	}
	return false
}
