package starlark

import (
	"context"
	"testing"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
)

type stubReader struct {
	guildCalls   int
	user         contract.UserDetailsRef
	member       contract.MemberDetailsRef
	warningCount int
	warnings     []contract.WarningRef
	plan         contract.ReminderPlanRef
}

func (reader *stubReader) GetUser(context.Context, string) (contract.UserDetailsRef, bool, error) {
	return reader.user, reader.user.User.ID != "", nil
}
func (reader *stubReader) GetMember(context.Context, string, string) (contract.MemberDetailsRef, bool, error) {
	return reader.member, reader.member.Member.User.ID != "", nil
}
func (reader *stubReader) GetGuild(_ context.Context, id string) (contract.GuildDetailsRef, bool, error) {
	reader.guildCalls++
	return contract.GuildDetailsRef{Guild: contract.GuildRef{ID: id, Name: "Mamacord"}}, true, nil
}
func (*stubReader) NormalizeTimezone(context.Context, string) (string, bool, error) {
	return "", false, nil
}
func (*stubReader) GetUserSettings(context.Context, string) (contract.UserSettingsRef, bool, error) {
	return contract.UserSettingsRef{}, false, nil
}
func (*stubReader) ListCheckIns(context.Context, string, int) ([]contract.CheckInRef, error) {
	return nil, nil
}
func (reader *stubReader) PlanReminder(context.Context, string, string, int64) (contract.ReminderPlanRef, bool, error) {
	return reader.plan, reader.plan.Schedule != "", nil
}
func (*stubReader) ListReminders(context.Context, string, int) ([]contract.ReminderRef, error) {
	return nil, nil
}
func (reader *stubReader) CountWarnings(context.Context, string, string) (int, error) {
	return reader.warningCount, nil
}
func (reader *stubReader) ListWarnings(context.Context, string, string, int) ([]contract.WarningRef, error) {
	return append([]contract.WarningRef(nil), reader.warnings...), nil
}

func TestContextReadsRequireExplicitCapability(t *testing.T) {
	t.Parallel()
	source := `load("@mamacord//api.star", "plugin", "cog", "slash_command", "reply")
def handle(ctx):
    guild = ctx.get_guild()
    return [reply(content=guild["name"])]
def setup(bot):
    bot.add_cog(cog(name="Reader", commands=[slash_command(name="reader", description="Reader", handler=handle)]))
PLUGIN=plugin(setup=setup)
`
	generation := buildTestGeneration(t, source)
	invocation := contract.Invocation{PluginID: "info", Generation: generation.ID(), Route: "command:slash:reader", Kind: contract.InvocationCommand, ResponseState: contract.ResponseUnacknowledged, Guild: &contract.GuildRef{ID: "10"}, Author: &contract.UserRef{ID: "1"}, Command: &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"reader"}}}
	reader := &stubReader{}
	if _, err := generation.InvokeWithServices(context.Background(), invocation, InvocationServices{Reader: reader}, nil); err == nil || !IsErrorKind(err, ErrorInvocation) {
		t.Fatalf("ungranted read: %v", err)
	}
	outcome, err := generation.InvokeWithServices(context.Background(), invocation, InvocationServices{Reader: reader, Capabilities: []contract.Capability{contract.CapabilityDiscordGuilds}}, nil)
	if err != nil {
		t.Fatalf("InvokeWithServices: %v", err)
	}
	if reader.guildCalls != 1 || outcome.Operations[0].(*contract.MessageOperation).Message.Content != "Mamacord" {
		t.Fatalf("calls=%d outcome=%#v", reader.guildCalls, outcome)
	}
}
