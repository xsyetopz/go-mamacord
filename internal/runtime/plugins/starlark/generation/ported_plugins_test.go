package generation

import (
	"context"
	"fmt"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/author"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/compile"
	contextapi "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/execution/context"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/internal/evaluation"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
)

func TestPortedBundlesCompileAndInitialize(t *testing.T) {
	t.Parallel()
	paths := map[string]string{
		"example":    filepath.Join("..", "..", "..", "..", "..", "examples", "plugins", "example", "bundles", "example-v0.1.0"),
		"fun":        filepath.Join("..", "..", "..", "..", "..", "plugins", "fun", "bundles", "release-v0.1.0"),
		"info":       filepath.Join("..", "..", "..", "..", "..", "plugins", "info", "bundles", "release-v0.1.0"),
		"manager":    filepath.Join("..", "..", "..", "..", "..", "plugins", "manager", "bundles", "release-v0.1.0"),
		"moderation": filepath.Join("..", "..", "..", "..", "..", "plugins", "moderation", "bundles", "release-v0.1.0"),
		"wellness":   filepath.Join("..", "..", "..", "..", "..", "plugins", "wellness", "bundles", "release-v0.1.0"),
	}
	for name, bundlePath := range paths {
		name, bundlePath := name, bundlePath
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bundle, err := compile.OpenDirBundle(bundlePath)
			if err != nil {
				t.Fatal(err)
			}
			compiled, err := compile.CompileBundle(context.Background(), bundle, evaluation.DefaultLimits())
			if err != nil {
				t.Fatal(err)
			}
			initialized, err := compiled.Initialize(context.Background(), author.AuthorAPI(), nil)
			if err != nil {
				t.Fatal(err)
			}
			generation, err := BuildInitialized(context.Background(), initialized, contract.GenerationID(name+"-test"), nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := generation.Definition().Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

type exampleLocalizer struct{}

func (exampleLocalizer) Localize(_ context.Context, request contextapi.LocalizationRequest) (string, error) {
	if request.MessageID == "example.counter" {
		fields, _ := request.Data.Object()
		for _, field := range fields {
			if field.Key == "Count" {
				count, _ := field.Value.Int()
				return fmt.Sprintf("Counter: %d", count), nil
			}
		}
	}
	if request.MessageID == "example.invalid.body" {
		return "invalid", nil
	}
	return request.MessageID, nil
}
func loadPortedExample(t *testing.T) *Generation {
	t.Helper()
	bundle, err := compile.OpenDirBundle(filepath.Join("..", "..", "..", "..", "..", "examples", "plugins", "example", "bundles", "example-v0.1.0"))
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := compile.CompileBundle(context.Background(), bundle, evaluation.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	initialized, err := compiled.Initialize(context.Background(), author.AuthorAPI(), nil)
	if err != nil {
		t.Fatal(err)
	}
	generation, err := BuildInitialized(context.Background(), initialized, "example-test", nil)
	if err != nil {
		t.Fatal(err)
	}
	return generation
}
func TestPortedExampleCounterUsesVersionedStateEffects(t *testing.T) {
	t.Parallel()
	generation := loadPortedExample(t)
	services := contextapi.InvocationServices{Localizer: exampleLocalizer{}, Capabilities: []contract.Capability{contract.CapabilityStorageKV}}
	base := contract.Invocation{InvocationIdentity: contract.InvocationIdentity{PluginID: "example", Generation: generation.ID()}, InvocationInteractionContext: contract.InvocationInteractionContext{ResponseState: contract.ResponseUnacknowledged}, InvocationActorContext: contract.InvocationActorContext{Guild: &contract.GuildRef{ID: "10"}, Channel: &contract.ChannelRef{ID: "20", GuildID: "10", Kind: contract.ChannelText}, Author: &contract.UserRef{ID: "30"}, Locale: "en-US"}, InvocationExecutionContext: contract.InvocationExecutionContext{State: []contract.StateEntry{{Key: "counter", Value: contract.IntValue(2), Version: 7}}}}
	command := base
	command.Route = "command:slash:example"
	command.Kind = contract.InvocationCommand
	command.Command = &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"example"}}
	outcome, err := generation.InvokeWithServices(context.Background(), command, services, nil)
	if err != nil {
		t.Fatal(err)
	}
	put := outcome.Operations[0].(*contract.KVPutOperation)
	if value, _ := put.Value.Int(); value != 3 || put.ExpectedVersion == nil || *put.ExpectedVersion != 7 {
		t.Fatalf("put=%#v", put)
	}
	if got := outcome.Operations[1].(*contract.MessageOperation).Message.Content; got != "Counter: 3" {
		t.Fatalf("content=%q", got)
	}
	component := base
	component.Route = "component:inc"
	component.Kind = contract.InvocationComponent
	component.ResponseState = contract.ResponseDeferredUpdate
	component.Component = &contract.ComponentInput{ID: "inc", Kind: contract.ComponentButton}
	outcome, err = generation.InvokeWithServices(context.Background(), component, services, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := outcome.Operations[1].(*contract.UpdateOperation); !ok {
		t.Fatalf("component outcome=%#v", outcome)
	}
	modal := base
	modal.Route = "modal:set_counter"
	modal.Kind = contract.InvocationModal
	modal.ModalOrigin = contract.ModalOriginComponent
	modal.Modal = &contract.ModalInput{ID: "set_counter", Fields: []contract.NamedString{{Name: "value", Value: "42"}}}
	outcome, err = generation.InvokeWithServices(context.Background(), modal, services, nil)
	if err != nil {
		t.Fatal(err)
	}
	if value, _ := outcome.Operations[0].(*contract.KVPutOperation).Value.Int(); value != 42 {
		t.Fatalf("modal outcome=%#v", outcome)
	}
	if _, ok := outcome.Operations[1].(*contract.UpdateOperation); !ok {
		t.Fatalf("modal terminal=%T", outcome.Operations[1])
	}
}

type passthroughLocalizer struct{}

func (passthroughLocalizer) Localize(_ context.Context, request contextapi.LocalizationRequest) (string, error) {
	return request.MessageID, nil
}
func loadPortedFun(t *testing.T) *Generation {
	t.Helper()
	bundle, err := compile.OpenDirBundle(filepath.Join("..", "..", "..", "..", "..", "plugins", "fun", "bundles", "release-v0.1.0"))
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := compile.CompileBundle(context.Background(), bundle, evaluation.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	initialized, err := compiled.Initialize(context.Background(), author.AuthorAPI(), nil)
	if err != nil {
		t.Fatal(err)
	}
	generation, err := BuildInitialized(context.Background(), initialized, "fun-test", nil)
	if err != nil {
		t.Fatal(err)
	}
	return generation
}
func TestPortedFunIsDeterministicAndUsesBoundedHTTP(t *testing.T) {
	t.Parallel()
	generation := loadPortedFun(t)
	base := contract.Invocation{InvocationIdentity: contract.InvocationIdentity{PluginID: "fun", Generation: generation.ID(), Kind: contract.InvocationCommand}, InvocationInteractionContext: contract.InvocationInteractionContext{ResponseState: contract.ResponseUnacknowledged}, InvocationActorContext: contract.InvocationActorContext{Guild: &contract.GuildRef{ID: "10"}, Author: &contract.UserRef{ID: "30"}, Locale: "en-US"}, InvocationExecutionContext: contract.InvocationExecutionContext{RandomSeed: 99}}
	roll := base
	roll.Route = "command:slash:roll"
	roll.Command = &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"roll"}, Options: []contract.OptionValue{{Name: "number", Kind: contract.OptionInteger, ScalarOptionValue: contract.ScalarOptionValue{Integer: 3}}, {Name: "sides", Kind: contract.OptionInteger, ScalarOptionValue: contract.ScalarOptionValue{Integer: 6}}, {Name: "modifier", Kind: contract.OptionInteger, ScalarOptionValue: contract.ScalarOptionValue{Integer: 1}}}}
	services := contextapi.InvocationServices{Localizer: passthroughLocalizer{}}
	first, err := generation.InvokeWithServices(context.Background(), roll, services, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := generation.InvokeWithServices(context.Background(), roll, services, nil)
	if err != nil {
		t.Fatal(err)
	}
	firstEmbed := first.Operations[0].(*contract.MessageOperation).Message.Embeds[0]
	secondEmbed := second.Operations[0].(*contract.MessageOperation).Message.Embeds[0]
	if firstEmbed.Footer == nil || secondEmbed.Footer == nil || firstEmbed.Footer.Text != secondEmbed.Footer.Text {
		t.Fatalf("non-deterministic embeds: %#v %#v", firstEmbed, secondEmbed)
	}
	client := &stubHTTPClient{}
	hug := base
	hug.Route = "command:slash:hug"
	hug.ResponseState = contract.ResponseDeferredCreate
	hug.Command = &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"hug"}, Options: []contract.OptionValue{{Name: "user", Kind: contract.OptionUser, ReferenceOptionValue: contract.ReferenceOptionValue{User: &contract.UserRef{ID: "40"}}}}}
	services = contextapi.InvocationServices{Localizer: passthroughLocalizer{}, HTTP: client, HTTPHosts: []string{"kawaii.red"}, Capabilities: []contract.Capability{contract.CapabilityNetworkHTTP}}
	outcome, err := generation.InvokeWithServices(context.Background(), hug, services, nil)
	if err != nil {
		t.Fatal(err)
	}
	if client.calls != 1 {
		t.Fatalf("HTTP calls=%d", client.calls)
	}
	if got := outcome.Operations[0].(*contract.EditResponseOperation).Patch.Embeds.Values[0].ImageURL; got != "https://cdn.example/gif.gif" {
		t.Fatalf("image=%q", got)
	}
}

func loadPortedInfo(t *testing.T) *Generation {
	t.Helper()
	bundle, err := compile.OpenDirBundle(filepath.Join("..", "..", "..", "..", "..", "plugins", "info", "bundles", "release-v0.1.0"))
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := compile.CompileBundle(context.Background(), bundle, evaluation.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	initialized, err := compiled.Initialize(context.Background(), author.AuthorAPI(), nil)
	if err != nil {
		t.Fatal(err)
	}
	generation, err := BuildInitialized(context.Background(), initialized, "info-test", nil)
	if err != nil {
		t.Fatal(err)
	}
	return generation
}
func TestPortedInfoUsesReadersOnlyForDiscordReads(t *testing.T) {
	t.Parallel()
	generation := loadPortedInfo(t)
	reader := &stubReader{user: contract.UserDetailsRef{User: contract.UserRef{ID: "40", Username: "target", Name: "Target"}, Mention: "<@40>", CreatedAt: 100}, member: contract.MemberDetailsRef{Member: contract.MemberRef{GuildID: "10", User: contract.UserRef{ID: "40"}, RoleIDs: []string{"50"}}, JoinedAt: 200}}
	services := contextapi.InvocationServices{Reader: reader, Localizer: passthroughLocalizer{}, Capabilities: []contract.Capability{contract.CapabilityDiscordUsers, contract.CapabilityDiscordMembers}}
	invocation := contract.Invocation{InvocationIdentity: contract.InvocationIdentity{PluginID: "info", Generation: generation.ID(), Route: "command:slash:lookup.user", Kind: contract.InvocationCommand}, InvocationInteractionContext: contract.InvocationInteractionContext{ResponseState: contract.ResponseDeferredCreate}, InvocationActorContext: contract.InvocationActorContext{Guild: &contract.GuildRef{ID: "10"}, Author: &contract.UserRef{ID: "30"}, Locale: "en-US"}, InvocationInput: contract.InvocationInput{Command: &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"lookup", "user"}, Options: []contract.OptionValue{{Name: "user", Kind: contract.OptionUser, ReferenceOptionValue: contract.ReferenceOptionValue{User: &contract.UserRef{ID: "40"}}}}}}}
	outcome, err := generation.InvokeWithServices(context.Background(), invocation, services, nil)
	if err != nil {
		t.Fatal(err)
	}
	edit, ok := outcome.Operations[0].(*contract.EditResponseOperation)
	if !ok || len(edit.Patch.Embeds.Values) != 1 || edit.Patch.Embeds.Values[0].Title != "Target" {
		t.Fatalf("outcome=%#v", outcome)
	}
	roleInvocation := contract.Invocation{InvocationIdentity: contract.InvocationIdentity{PluginID: "info", Generation: generation.ID(), Route: "command:slash:lookup.role", Kind: contract.InvocationCommand}, InvocationInteractionContext: contract.InvocationInteractionContext{ResponseState: contract.ResponseUnacknowledged}, InvocationActorContext: contract.InvocationActorContext{Guild: &contract.GuildRef{ID: "10"}, Author: &contract.UserRef{ID: "30"}, Locale: "en-US"}, InvocationInput: contract.InvocationInput{Command: &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"lookup", "role"}, Options: []contract.OptionValue{{Name: "role", Kind: contract.OptionRole, ReferenceOptionValue: contract.ReferenceOptionValue{Role: &contract.RoleRef{RoleIdentity: contract.RoleIdentity{ID: "50", GuildID: "10", Name: "Helper"}, RolePresentation: contract.RolePresentation{Mention: "<@&50>"}, CreatedAt: 100}}}}}}}
	outcome, err = generation.InvokeWithServices(context.Background(), roleInvocation, contextapi.InvocationServices{Localizer: passthroughLocalizer{}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := outcome.Operations[0].(*contract.MessageOperation).Message.Embeds[0].Title; got != "Helper" {
		t.Fatalf("role title=%q", got)
	}
}

func loadPortedManager(t *testing.T) *Generation {
	t.Helper()
	bundle, err := compile.OpenDirBundle(filepath.Join("..", "..", "..", "..", "..", "plugins", "manager", "bundles", "release-v0.1.0"))
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := compile.CompileBundle(context.Background(), bundle, evaluation.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	initialized, err := compiled.Initialize(context.Background(), author.AuthorAPI(), nil)
	if err != nil {
		t.Fatal(err)
	}
	generation, err := BuildInitialized(context.Background(), initialized, "manager-test", nil)
	if err != nil {
		t.Fatal(err)
	}
	return generation
}
func TestPortedManagerReturnsGuardedAuthorizedEffects(t *testing.T) {
	t.Parallel()
	generation := loadPortedManager(t)
	invocation := contract.Invocation{InvocationIdentity: contract.InvocationIdentity{PluginID: "manager", Generation: generation.ID(), Route: "command:slash:slowmode", Kind: contract.InvocationCommand}, InvocationInteractionContext: contract.InvocationInteractionContext{ResponseState: contract.ResponseDeferredCreate}, InvocationActorContext: contract.InvocationActorContext{Guild: &contract.GuildRef{ID: "10"}, Channel: &contract.ChannelRef{ID: "20", GuildID: "10", Kind: contract.ChannelText}, Author: &contract.UserRef{ID: "30"}, Locale: "en-US"}, InvocationInput: contract.InvocationInput{Command: &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"slowmode"}}}}
	services := contextapi.InvocationServices{Localizer: passthroughLocalizer{}, Capabilities: []contract.Capability{contract.CapabilityDiscordChannels}}
	outcome, err := generation.InvokeWithServices(context.Background(), invocation, services, nil)
	if err != nil {
		t.Fatal(err)
	}
	guarded, ok := outcome.Operations[0].(*contract.GuardedOperation)
	if !ok {
		t.Fatalf("operation=%T", outcome.Operations[0])
	}
	slowmode, ok := guarded.Operation.(*contract.SetSlowmodeOperation)
	if !ok || slowmode.Seconds != 0 || slowmode.ChannelID != "20" {
		t.Fatalf("slowmode=%#v", guarded.Operation)
	}
	if _, ok := guarded.Failure.(*contract.EditResponseOperation); !ok {
		t.Fatalf("failure=%T", guarded.Failure)
	}
	if _, ok := outcome.Operations[1].(*contract.EditResponseOperation); !ok {
		t.Fatalf("success=%T", outcome.Operations[1])
	}
	if _, err := generation.InvokeWithServices(context.Background(), invocation, contextapi.InvocationServices{Localizer: passthroughLocalizer{}}, nil); err == nil || !evaluation.IsErrorKind(err, evaluation.ErrorValidation) {
		t.Fatalf("missing capability: %v", err)
	}
}

func loadPortedModeration(t *testing.T) *Generation {
	t.Helper()
	bundle, err := compile.OpenDirBundle(filepath.Join("..", "..", "..", "..", "..", "plugins", "moderation", "bundles", "release-v0.1.0"))
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := compile.CompileBundle(context.Background(), bundle, evaluation.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	initialized, err := compiled.Initialize(context.Background(), author.AuthorAPI(), nil)
	if err != nil {
		t.Fatal(err)
	}
	generation, err := BuildInitialized(context.Background(), initialized, "moderation-test", nil)
	if err != nil {
		t.Fatal(err)
	}
	return generation
}
func TestPortedModerationReturnsTransactionalFailurePolicies(t *testing.T) {
	t.Parallel()
	generation := loadPortedModeration(t)
	config, err := contract.ObjectValue([]contract.Field{{Key: "warning_limit", Value: contract.IntValue(3)}, {Key: "timeout_threshold", Value: contract.IntValue(1)}, {Key: "timeout_minutes", Value: contract.IntValue(10)}})
	if err != nil {
		t.Fatal(err)
	}
	invocation := contract.Invocation{InvocationIdentity: contract.InvocationIdentity{PluginID: "moderation", Generation: generation.ID(), Route: "command:slash:warn", Kind: contract.InvocationCommand}, InvocationInteractionContext: contract.InvocationInteractionContext{ResponseState: contract.ResponseDeferredCreate}, InvocationActorContext: contract.InvocationActorContext{Guild: &contract.GuildRef{ID: "10"}, Channel: &contract.ChannelRef{ID: "20", GuildID: "10", Kind: contract.ChannelText}, Author: &contract.UserRef{ID: "30"}, Locale: "en-US"}, InvocationExecutionContext: contract.InvocationExecutionContext{NowUnix: 1000, State: []contract.StateEntry{{Key: "guild_config", Value: config, Version: 1}}}, InvocationInput: contract.InvocationInput{Command: &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"warn"}, Options: []contract.OptionValue{{Name: "user", Kind: contract.OptionUser, ReferenceOptionValue: contract.ReferenceOptionValue{User: &contract.UserRef{ID: "40", Name: "Target"}}}, {Name: "reason", Kind: contract.OptionString, ScalarOptionValue: contract.ScalarOptionValue{String: "Be kind"}}}}}}
	services := contextapi.InvocationServices{Reader: &stubReader{}, Localizer: passthroughLocalizer{}, Capabilities: []contract.Capability{contract.CapabilityStorageKV, contract.CapabilityStorageWarnings, contract.CapabilityStorageAudit, contract.CapabilityDiscordMessages, contract.CapabilityDiscordMembers}}
	outcome, err := generation.InvokeWithServices(context.Background(), invocation, services, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(outcome.Operations) != 6 {
		t.Fatalf("operations=%d", len(outcome.Operations))
	}
	first, ok := outcome.Operations[0].(*contract.GuardedOperation)
	if !ok {
		t.Fatalf("first=%T", outcome.Operations[0])
	}
	warning, ok := first.Operation.(*contract.CreateWarningOperation)
	if !ok || warning.UserID != "40" || warning.Reason != "Be kind" {
		t.Fatalf("warning=%#v", first.Operation)
	}
	timeout := outcome.Operations[2].(*contract.GuardedOperation).Operation.(*contract.TimeoutMemberOperation)
	if timeout.UntilUnix != 1600 {
		t.Fatalf("timeout=%#v", timeout)
	}
	if dm, ok := outcome.Operations[4].(*contract.BestEffortOperation); !ok {
		t.Fatalf("dm=%T", outcome.Operations[4])
	} else if _, ok := dm.Operation.(*contract.SendDMOperation); !ok {
		t.Fatalf("dm operation=%T", dm.Operation)
	}
	if _, ok := outcome.Operations[5].(*contract.EditResponseOperation); !ok {
		t.Fatalf("response=%T", outcome.Operations[5])
	}
}

func loadPortedWellness(t *testing.T) *Generation {
	t.Helper()
	bundle, err := compile.OpenDirBundle(filepath.Join("..", "..", "..", "..", "..", "plugins", "wellness", "bundles", "release-v0.1.0"))
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := compile.CompileBundle(context.Background(), bundle, evaluation.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	initialized, err := compiled.Initialize(context.Background(), author.AuthorAPI(), nil)
	if err != nil {
		t.Fatal(err)
	}
	generation, err := BuildInitialized(context.Background(), initialized, "wellness-test", nil)
	if err != nil {
		t.Fatal(err)
	}
	return generation
}
func TestPortedWellnessCreatesDeterministicReminderEffect(t *testing.T) {
	t.Parallel()
	generation := loadPortedWellness(t)
	reader := &stubReader{plan: contract.ReminderPlanRef{Schedule: "0 9 * * *", NextRunAt: 2000}}
	services := contextapi.InvocationServices{Reader: reader, Localizer: passthroughLocalizer{}, Capabilities: []contract.Capability{contract.CapabilityStorageReminders}}
	invocation := contract.Invocation{InvocationIdentity: contract.InvocationIdentity{PluginID: "wellness", Generation: generation.ID(), Route: "command:slash:remind.create", Kind: contract.InvocationCommand}, InvocationInteractionContext: contract.InvocationInteractionContext{ResponseState: contract.ResponseDeferredCreate}, InvocationActorContext: contract.InvocationActorContext{Author: &contract.UserRef{ID: "30"}, Locale: "en-US"}, InvocationExecutionContext: contract.InvocationExecutionContext{NowUnix: 1000, RandomSeed: 123}, InvocationInput: contract.InvocationInput{Command: &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"remind", "create"}, Options: []contract.OptionValue{{Name: "schedule", Kind: contract.OptionString, ScalarOptionValue: contract.ScalarOptionValue{String: "0 9 * * *"}}, {Name: "kind", Kind: contract.OptionString, ScalarOptionValue: contract.ScalarOptionValue{String: "hydrate"}}}}}}
	first, err := generation.InvokeWithServices(context.Background(), invocation, services, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := generation.InvokeWithServices(context.Background(), invocation, services, nil)
	if err != nil {
		t.Fatal(err)
	}
	firstReminder := first.Operations[0].(*contract.GuardedOperation).Operation.(*contract.CreateReminderOperation)
	secondReminder := second.Operations[0].(*contract.GuardedOperation).Operation.(*contract.CreateReminderOperation)
	if firstReminder.ReminderID == "" || firstReminder.ReminderID != secondReminder.ReminderID || firstReminder.NextRunAt != 2000 {
		t.Fatalf("reminders=%#v %#v", firstReminder, secondReminder)
	}
	if _, err := uuid.Parse(firstReminder.ReminderID); err != nil {
		t.Fatalf("reminder ID %q: %v", firstReminder.ReminderID, err)
	}
	if _, ok := first.Operations[1].(*contract.EditResponseOperation); !ok {
		t.Fatalf("response=%T", first.Operations[1])
	}
}

func TestOfficialDefinitionsRetainLocalizedDescriptionsAndManagerRolePermission(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"fun", "info", "manager", "moderation", "wellness"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bundle, err := compile.OpenDirBundle(filepath.Join("..", "..", "..", "..", "..", "plugins", name, "bundles", "release-v0.1.0"))
			if err != nil {
				t.Fatal(err)
			}
			compiled, err := compile.CompileBundle(context.Background(), bundle, evaluation.DefaultLimits())
			if err != nil {
				t.Fatal(err)
			}
			initialized, err := compiled.Initialize(context.Background(), author.AuthorAPI(), nil)
			if err != nil {
				t.Fatal(err)
			}
			generation, err := BuildInitialized(context.Background(), initialized, contract.GenerationID(name+"-descriptions"), nil)
			if err != nil {
				t.Fatal(err)
			}
			var check func([]contract.CommandDefinition)
			check = func(commands []contract.CommandDefinition) {
				for _, command := range commands {
					if command.DescriptionID == "" {
						t.Errorf("command %s lacks description_id", command.Route)
					}
					for _, option := range command.Options {
						if option.DescriptionID == "" {
							t.Errorf("option %s/%s lacks description_id", command.Route, option.Name)
						}
					}
					check(command.Children)
				}
			}
			definition := generation.Definition()
			for _, cog := range definition.Cogs {
				check(cog.Commands)
				if name == "manager" {
					for _, command := range cog.Commands {
						if command.Name == "roles" {
							found := false
							for _, permission := range command.DefaultMemberPermissions {
								found = found || permission == contract.PermissionManageRoles
							}
							if !found {
								t.Error("roles lacks manage_roles permission")
							}
						}
					}
				}
			}
		})
	}
}

func TestPortedModerationDisabledConfigCoversEveryRoute(t *testing.T) {
	generation := loadPortedModeration(t)
	disabled, err := contract.ObjectValue([]contract.Field{{Key: "enabled", Value: contract.BoolValue(false)}})
	if err != nil {
		t.Fatal(err)
	}
	base := contract.Invocation{InvocationIdentity: contract.InvocationIdentity{PluginID: "moderation", Generation: generation.ID()}, InvocationActorContext: contract.InvocationActorContext{Guild: &contract.GuildRef{ID: "10"}, Channel: &contract.ChannelRef{ID: "20", GuildID: "10", Kind: contract.ChannelText}, Author: &contract.UserRef{ID: "30"}, Locale: "en-US"}, InvocationExecutionContext: contract.InvocationExecutionContext{NowUnix: 1000, State: []contract.StateEntry{{Key: "guild_config", Value: disabled, Version: 1}}}}
	tests := []contract.Invocation{base.DeepClone(), base.DeepClone(), base.DeepClone()}
	tests[0].Route = "command:slash:warn"
	tests[0].Kind = contract.InvocationCommand
	tests[0].ResponseState = contract.ResponseDeferredCreate
	tests[0].Command = &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"warn"}, Options: []contract.OptionValue{{Name: "user", Kind: contract.OptionUser, ReferenceOptionValue: contract.ReferenceOptionValue{User: &contract.UserRef{ID: "40"}}}, {Name: "reason", Kind: contract.OptionString, ScalarOptionValue: contract.ScalarOptionValue{String: "reason"}}}}
	tests[1].Route = "command:slash:unwarn"
	tests[1].Kind = contract.InvocationCommand
	tests[1].ResponseState = contract.ResponseDeferredCreate
	tests[1].Command = &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"unwarn"}, Options: []contract.OptionValue{{Name: "user", Kind: contract.OptionUser, ReferenceOptionValue: contract.ReferenceOptionValue{User: &contract.UserRef{ID: "40"}}}}}
	tests[2].Route = "component:unwarn_select"
	tests[2].Kind = contract.InvocationComponent
	tests[2].ResponseState = contract.ResponseDeferredUpdate
	tests[2].Component = &contract.ComponentInput{ID: "unwarn_select", Kind: contract.ComponentStringSelect, Values: []contract.OptionValue{{Name: "value", Kind: contract.OptionString, ScalarOptionValue: contract.ScalarOptionValue{String: "warning|30|40|1000"}}}}
	for _, invocation := range tests {
		outcome, err := generation.InvokeWithServices(context.Background(), invocation, contextapi.InvocationServices{Localizer: passthroughLocalizer{}, Capabilities: []contract.Capability{contract.CapabilityStorageKV}}, nil)
		if err != nil {
			t.Fatalf("route %s: %v", invocation.Route, err)
		}
		if len(outcome.Operations) != 1 {
			t.Fatalf("route %s operations=%d", invocation.Route, len(outcome.Operations))
		}
		switch invocation.Kind {
		case contract.InvocationComponent:
			if _, ok := outcome.Operations[0].(*contract.UpdateOperation); !ok {
				t.Fatalf("route %s operation=%T", invocation.Route, outcome.Operations[0])
			}
		default:
			if _, ok := outcome.Operations[0].(*contract.EditResponseOperation); !ok {
				t.Fatalf("route %s operation=%T", invocation.Route, outcome.Operations[0])
			}
		}
	}
}

func TestPortedManagerRejectsKnownInvalidMediaBeforeEffects(t *testing.T) {
	generation := loadPortedManager(t)
	base := contract.Invocation{InvocationIdentity: contract.InvocationIdentity{PluginID: "manager", Generation: generation.ID(), Kind: contract.InvocationCommand}, InvocationInteractionContext: contract.InvocationInteractionContext{ResponseState: contract.ResponseDeferredCreate}, InvocationActorContext: contract.InvocationActorContext{Guild: &contract.GuildRef{ID: "10"}, Channel: &contract.ChannelRef{ID: "20", GuildID: "10", Kind: contract.ChannelText}, Author: &contract.UserRef{ID: "30"}, Locale: "en-US"}}
	create := base.DeepClone()
	create.Route = "command:slash:emojis.create"
	create.Command = &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"emojis", "create"}, Options: []contract.OptionValue{{Name: "name", Kind: contract.OptionString, ScalarOptionValue: contract.ScalarOptionValue{String: "wave"}}, {Name: "file", Kind: contract.OptionAttachment, ReferenceOptionValue: contract.ReferenceOptionValue{Attachment: &contract.AttachmentRef{ID: "50", Filename: "wave.txt", ContentType: "text/plain", URL: "https://cdn.discordapp.com/wave.txt", Size: 10, Width: 32, Height: 32}}}}}
	outcome, err := generation.InvokeWithServices(context.Background(), create, contextapi.InvocationServices{Localizer: passthroughLocalizer{}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(outcome.Operations) != 1 {
		t.Fatalf("operations=%d", len(outcome.Operations))
	}
	response := outcome.Operations[0].(*contract.EditResponseOperation)
	if len(response.Patch.Embeds.Values) != 1 || response.Patch.Embeds.Values[0].Description != "mgr.emojis.bad_extension" {
		t.Fatalf("response=%#v", response)
	}
	edit := base.DeepClone()
	edit.Route = "command:slash:emojis.edit"
	edit.Command = &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"emojis", "edit"}, Options: []contract.OptionValue{{Name: "emoji", Kind: contract.OptionString, ScalarOptionValue: contract.ScalarOptionValue{String: "not-an-emoji"}}, {Name: "name", Kind: contract.OptionString, ScalarOptionValue: contract.ScalarOptionValue{String: "wave"}}}}
	outcome, err = generation.InvokeWithServices(context.Background(), edit, contextapi.InvocationServices{Localizer: passthroughLocalizer{}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	response = outcome.Operations[0].(*contract.EditResponseOperation)
	if response.Patch.Embeds.Values[0].Description != "mgr.emojis.invalid_emoji" {
		t.Fatalf("response=%#v", response)
	}
}

type stubHTTPClient struct {
	calls    int
	url      string
	maxBytes int64
}

func (client *stubHTTPClient) GetJSON(_ context.Context, rawURL string, maxBytes int64) (contract.Value, bool, error) {
	client.calls++
	client.url = rawURL
	client.maxBytes = maxBytes
	value, err := contract.ObjectValue([]contract.Field{{Key: "response", Value: contract.StringValue("https://cdn.example/gif.gif")}})
	return value, true, err
}

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
