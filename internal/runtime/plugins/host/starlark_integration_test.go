package pluginhost

import (
	"context"
	"encoding/json"
	pluginmanifest "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/manifest"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/xsyetopz/go-mamacord/internal/bundles"
	"github.com/xsyetopz/go-mamacord/internal/i18n"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	storage "github.com/xsyetopz/go-mamacord/internal/storage"
)

type recordingDiscordBridge struct {
	mu         sync.Mutex
	operations []contract.Operation
}

func (*recordingDiscordBridge) GetUser(context.Context, string) (contract.UserDetailsRef, bool, error) {
	return contract.UserDetailsRef{}, false, nil
}
func (*recordingDiscordBridge) GetMember(context.Context, string, string) (contract.MemberDetailsRef, bool, error) {
	return contract.MemberDetailsRef{}, false, nil
}
func (*recordingDiscordBridge) GetGuild(context.Context, string) (contract.GuildDetailsRef, bool, error) {
	return contract.GuildDetailsRef{}, false, nil
}
func (bridge *recordingDiscordBridge) Execute(_ context.Context, _ EffectScope, operation contract.Operation) error {
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	bridge.operations = append(bridge.operations, cloneContractOperation(operation))
	return nil
}
func (bridge *recordingDiscordBridge) count() int {
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	return len(bridge.operations)
}
func loadProductionStarlarkHost(t *testing.T, bridge DiscordBridge) *Host {
	t.Helper()
	root := filepath.Join("..", "..", "..", "..")
	registry, err := i18n.LoadCore(filepath.Join(root, "locales"))
	if err != nil {
		t.Fatal(err)
	}
	host, err := NewHost(Options{BundleOptions: BundleOptions{Dirs: []string{filepath.Join(root, "plugins")}}, AuthorityOptions: AuthorityOptions{AllowUnsignedPlugin: true, PermissionsFile: filepath.Join(root, "config", "permissions.json"), TrustedKeysFile: filepath.Join(root, "config", "trusted_keys.json")}, RuntimeOptions: RuntimeOptions{Bridge: Bridge{Discord: bridge}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), I18n: &registry}})
	if err != nil {
		t.Fatal(err)
	}
	if err := host.LoadAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := host.Close(ctx); err != nil {
			t.Errorf("close host: %v", err)
		}
	})
	return host
}
func TestProductionStarlarkHostLoadsAndInvokesFun(t *testing.T) {
	bridge := &recordingDiscordBridge{}
	host := loadProductionStarlarkHost(t, bridge)
	host.i18n.ResetPluginLocales()
	plan, err := host.PlanCommand("slash", "flip", []string{"flip"})
	if err != nil {
		t.Fatal(err)
	}
	terminal, err := host.Run(context.Background(), "fun", contract.Invocation{InvocationIdentity: contract.InvocationIdentity{Route: plan.Route, Kind: contract.InvocationCommand}, InvocationInteractionContext: contract.InvocationInteractionContext{ResponseState: contract.ResponseUnacknowledged}, InvocationActorContext: contract.InvocationActorContext{Author: &contract.UserRef{ID: "175928847299117063", Username: "tester", Name: "Tester"}, Locale: "en-US"}, InvocationInput: contract.InvocationInput{Command: &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"flip"}}}})
	if err != nil {
		t.Fatal(err)
	}
	message, ok := terminal.(*contract.MessageOperation)
	if !ok || len(message.Message.Embeds) != 1 {
		t.Fatalf("terminal=%#v", terminal)
	}
	if strings.Contains(message.Message.Embeds[0].Description, "<no value>") {
		t.Fatalf("persona template data missing: %q", message.Message.Embeds[0].Description)
	}
	if bridge.count() != 0 {
		t.Fatalf("unexpected Discord effects: %d", bridge.count())
	}
}
func TestProductionStarlarkHostEnforcesCommandPermissionsBeforeEffects(t *testing.T) {
	bridge := &recordingDiscordBridge{}
	host := loadProductionStarlarkHost(t, bridge)
	plan, err := host.PlanCommand("slash", "slowmode", []string{"slowmode"})
	if err != nil {
		t.Fatal(err)
	}
	invocation := contract.Invocation{InvocationIdentity: contract.InvocationIdentity{Route: plan.Route, Kind: contract.InvocationCommand}, InvocationInteractionContext: contract.InvocationInteractionContext{ResponseState: contract.ResponseDeferredCreate}, InvocationActorContext: contract.InvocationActorContext{Guild: &contract.GuildRef{ID: "175928847299117063", Name: "Guild"}, Channel: &contract.ChannelRef{ID: "175928847299117064", GuildID: "175928847299117063", Kind: contract.ChannelText}, Author: &contract.UserRef{ID: "175928847299117065"}, Member: &contract.MemberRef{GuildID: "175928847299117063", User: contract.UserRef{ID: "175928847299117065"}}, Locale: "en-US"}, InvocationInput: contract.InvocationInput{Command: &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"slowmode"}, Options: []contract.OptionValue{{Name: "seconds", Kind: contract.OptionInteger, ScalarOptionValue: contract.ScalarOptionValue{Integer: 0}}}}}}
	terminal, err := host.Run(context.Background(), "manager", invocation)
	if err != nil {
		t.Fatal(err)
	}
	if terminal != nil || bridge.count() != 0 {
		t.Fatalf("denied invocation terminal=%T effects=%d", terminal, bridge.count())
	}
	invocation.Member.Permissions = []contract.MemberPermission{contract.PermissionManageChannels}
	terminal, err = host.Run(context.Background(), "manager", invocation)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := terminal.(*contract.EditResponseOperation); !ok {
		t.Fatalf("terminal=%T", terminal)
	}
	if bridge.count() != 1 {
		t.Fatalf("effects=%d", bridge.count())
	}
}

type conflictingKV struct {
	mu       sync.Mutex
	value    string
	version  uint64
	exists   bool
	attempts int
	reads    int
}

func (kv *conflictingKV) GetPluginKV(context.Context, uint64, string, string) (string, bool, error) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	return kv.value, kv.exists, nil
}
func (kv *conflictingKV) PutPluginKV(_ context.Context, _ uint64, _, _, value string) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	kv.value = value
	kv.version++
	kv.exists = true
	return nil
}
func (kv *conflictingKV) DeletePluginKV(context.Context, uint64, string, string) error { return nil }
func (kv *conflictingKV) GetPluginKVVersioned(context.Context, uint64, string, string) (storage.PluginKVValue, bool, error) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	kv.reads++
	return storage.PluginKVValue{ValueJSON: kv.value, Version: kv.version}, kv.exists, nil
}
func (kv *conflictingKV) CompareAndSwapPluginKV(_ context.Context, _ uint64, _, _, value string, expected uint64) (uint64, bool, error) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	kv.attempts++
	if kv.attempts == 1 {
		kv.value = "5"
		kv.version = 1
		kv.exists = true
		return 0, false, nil
	}
	if !kv.exists || kv.version != expected {
		return 0, false, nil
	}
	kv.version++
	kv.value = value
	return kv.version, true, nil
}
func (kv *conflictingKV) DeletePluginKVVersion(context.Context, uint64, string, string, uint64) (bool, error) {
	return false, nil
}
func TestProductionStarlarkHostRetriesVersionedStateConflict(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	registry, err := i18n.LoadCore(filepath.Join(root, "locales"))
	if err != nil {
		t.Fatal(err)
	}
	kv := &conflictingKV{}
	host, err := NewHost(Options{BundleOptions: BundleOptions{Dirs: []string{filepath.Join(root, "examples", "plugins")}}, AuthorityOptions: AuthorityOptions{AllowUnsignedPlugin: true, PermissionsFile: filepath.Join(root, "config", "permissions.json")}, RuntimeOptions: RuntimeOptions{Store: hostStoreStub{kv: kv}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), I18n: &registry}})
	if err != nil {
		t.Fatal(err)
	}
	if err = host.LoadAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = host.Close(ctx)
	})
	plan, err := host.PlanCommand("slash", "example", []string{"example"})
	if err != nil {
		t.Fatal(err)
	}
	terminal, err := host.Run(context.Background(), "example", contract.Invocation{InvocationIdentity: contract.InvocationIdentity{Route: plan.Route, Kind: contract.InvocationCommand}, InvocationInteractionContext: contract.InvocationInteractionContext{ResponseState: contract.ResponseUnacknowledged}, InvocationActorContext: contract.InvocationActorContext{Guild: &contract.GuildRef{ID: "175928847299117063"}, Channel: &contract.ChannelRef{ID: "175928847299117064", GuildID: "175928847299117063", Kind: contract.ChannelText}, Author: &contract.UserRef{ID: "175928847299117065"}, Locale: "en-US"}, InvocationInput: contract.InvocationInput{Command: &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"example"}}}})
	if err != nil {
		t.Fatal(err)
	}
	message, ok := terminal.(*contract.MessageOperation)
	if !ok {
		t.Fatalf("terminal=%T", terminal)
	}
	if !strings.Contains(message.Message.Content, "6") {
		t.Fatalf("content=%q", message.Message.Content)
	}
	kv.mu.Lock()
	defer kv.mu.Unlock()
	if kv.attempts != 2 || kv.reads != 2 || kv.value != "6" {
		t.Fatalf("attempts=%d reads=%d value=%q version=%d", kv.attempts, kv.reads, kv.value, kv.version)
	}
}

func TestProductionInfoLookupPlansPrivateDeferredResponses(t *testing.T) {
	host := loadProductionStarlarkHost(t, &recordingDiscordBridge{})
	for _, path := range [][]string{{"lookup", "user"}, {"lookup", "guild"}} {
		plan, err := host.PlanCommand("slash", path[0], path)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Defer != contract.DeferCreate || !plan.Ephemeral {
			t.Errorf("path=%v defer=%q ephemeral=%v", path, plan.Defer, plan.Ephemeral)
		}
	}
}

func writeCustomCheckPlugin(t *testing.T, root string) {
	t.Helper()
	pluginRoot := filepath.Join(root, "checks")
	bundleDir := filepath.Join(pluginRoot, "bundles", "test-v0.1.0")
	if err := os.MkdirAll(filepath.Join(bundleDir, "locales", "en-US"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest, err := json.Marshal(pluginmanifest.StarlarkManifest{Schema: pluginmanifest.StarlarkManifestSchema, ID: "checks", Name: "Checks", Version: "0.1.0", Entrypoint: pluginmanifest.StarlarkEntrypoint, Permissions: pluginmanifest.StarlarkManifestPermissions{Network: pluginmanifest.StarlarkManifestNetworkPermissions{Hosts: []string{}}}, Locales: pluginmanifest.StarlarkManifestLocales{Default: "en-US", Supported: []string{"en-US"}}, StateKeys: []string{}, Assets: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	source := `load("@mamacord//api.star", "autocomplete_choice", "autocomplete_choices", "cog", "component", "custom_check", "integer_option", "plugin", "reply", "slash_command", "update")
def deny(ctx): return reply(content="denied", ephemeral=True)
def complete(ctx): return [autocomplete_choices([autocomplete_choice(name="should-not-run", value=1)])]
def command(ctx): return [reply(content="should-not-run")]
def component_handler(ctx): return [update(content="should-not-run")]
def setup(bot): bot.add_cog(cog(name="Checks", commands=[slash_command(name="checked", description="Checked", handler=command, checks=[custom_check(id="deny", handler=deny)], options=[integer_option(name="value", description="Value", autocomplete=complete)]), slash_command(name="protected", description="Protected", handler=command, permissions=["manage_roles"], checks=[custom_check(id="deny_protected", handler=deny)])], components=[component(id="checked", handler=component_handler, kinds=["button"], defer="update", checks=[custom_check(id="deny_component", handler=deny)])]))
PLUGIN=plugin(setup=setup)
`
	for path, data := range map[string][]byte{filepath.Join(bundleDir, "plugin.json"): manifest, filepath.Join(bundleDir, "plugin.star"): []byte(source), filepath.Join(bundleDir, "locales", "en-US", "messages.json"): []byte("[]\n")} {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := bundles.NewLocalRepository().WriteState(pluginRoot, bundles.State{ActiveRelativeDir: filepath.Join("bundles", "test-v0.1.0"), Revision: "0.1.0"}); err != nil {
		t.Fatal(err)
	}
}
func TestHostCustomCheckDenialsRespectOriginalRouteProtocol(t *testing.T) {
	dir := t.TempDir()
	writeCustomCheckPlugin(t, dir)
	host, err := NewHost(Options{BundleOptions: BundleOptions{Dirs: []string{dir}}, AuthorityOptions: AuthorityOptions{AllowUnsignedPlugin: true}, RuntimeOptions: RuntimeOptions{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}})
	if err != nil {
		t.Fatal(err)
	}
	if err = host.LoadAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = host.Close(ctx)
	})
	commandPlan, err := host.PlanCommand("slash", "checked", []string{"checked"})
	if err != nil {
		t.Fatal(err)
	}
	base := contract.Invocation{InvocationActorContext: contract.InvocationActorContext{Author: &contract.UserRef{ID: "175928847299117063"}}}
	command := base
	command.Route = commandPlan.Route
	command.Kind = contract.InvocationCommand
	command.ResponseState = contract.ResponseUnacknowledged
	command.Command = &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"checked"}}
	terminal, err := host.Run(context.Background(), "checks", command)
	if err != nil {
		t.Fatal(err)
	}
	if message, ok := terminal.(*contract.MessageOperation); !ok || message.Message.Content != "denied" {
		t.Fatalf("command denial=%#v", terminal)
	}
	autocompletePlan, err := host.PlanAutocomplete("slash", "checked", []string{"checked"}, "value")
	if err != nil {
		t.Fatal(err)
	}
	autocomplete := base
	autocomplete.Route = autocompletePlan.Route
	autocomplete.Kind = contract.InvocationAutocomplete
	autocomplete.Autocomplete = &contract.AutocompleteInput{Path: []string{"checked"}, Option: "value", Focused: contract.OptionValue{Name: "value", Kind: contract.OptionInteger, ScalarOptionValue: contract.ScalarOptionValue{Integer: 1}}}
	terminal, err = host.Run(context.Background(), "checks", autocomplete)
	if err != nil {
		t.Fatal(err)
	}
	choices, ok := terminal.(*contract.AutocompleteChoicesOperation)
	if !ok || len(choices.Choices) != 0 {
		t.Fatalf("autocomplete denial=%#v", terminal)
	}
	componentPlan, err := host.PlanComponent("checks", "checked")
	if err != nil {
		t.Fatal(err)
	}
	component := base
	component.Route = componentPlan.Route
	component.Kind = contract.InvocationComponent
	component.ResponseState = contract.ResponseDeferredUpdate
	component.Component = &contract.ComponentInput{ID: "checked", Kind: contract.ComponentButton}
	terminal, err = host.Run(context.Background(), "checks", component)
	if err != nil {
		t.Fatal(err)
	}
	update, ok := terminal.(*contract.UpdateOperation)
	if !ok || !update.Patch.Content.Set || update.Patch.Content.Value != "denied" {
		t.Fatalf("component denial=%#v", terminal)
	}
	component.ResponseState = contract.ResponseUnacknowledged
	admission, denial, err := host.Admit(context.Background(), "checks", component)
	if err != nil {
		t.Fatal(err)
	}
	if admission != nil {
		t.Fatal("denied component was admitted")
	}
	message, ok := denial.(*contract.MessageOperation)
	if !ok || !message.Ephemeral || message.Message.Content != "denied" {
		t.Fatalf("pre-defer denial=%#v", denial)
	}
}

func TestProductionManagerRoleGroupPermissionIsEnforcedAtRuntime(t *testing.T) {
	bridge := &recordingDiscordBridge{}
	host := loadProductionStarlarkHost(t, bridge)
	plan, err := host.PlanCommand("slash", "roles", []string{"roles", "delete"})
	if err != nil {
		t.Fatal(err)
	}
	invocation := contract.Invocation{InvocationIdentity: contract.InvocationIdentity{Route: plan.Route, Kind: contract.InvocationCommand}, InvocationInteractionContext: contract.InvocationInteractionContext{ResponseState: contract.ResponseDeferredCreate}, InvocationActorContext: contract.InvocationActorContext{Guild: &contract.GuildRef{ID: "175928847299117063"}, Channel: &contract.ChannelRef{ID: "175928847299117064", GuildID: "175928847299117063", Kind: contract.ChannelText}, Author: &contract.UserRef{ID: "175928847299117065"}, Member: &contract.MemberRef{GuildID: "175928847299117063", User: contract.UserRef{ID: "175928847299117065"}}, Locale: "en-US"}, InvocationInput: contract.InvocationInput{Command: &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"roles", "delete"}, Options: []contract.OptionValue{{Name: "role", Kind: contract.OptionRole, ReferenceOptionValue: contract.ReferenceOptionValue{Role: &contract.RoleRef{RoleIdentity: contract.RoleIdentity{ID: "175928847299117066", GuildID: "175928847299117063", Name: "old"}, RolePresentation: contract.RolePresentation{Mention: "<@&175928847299117066>"}}}}}}}}
	terminal, err := host.Run(context.Background(), "manager", invocation)
	if err != nil {
		t.Fatal(err)
	}
	if terminal != nil || bridge.count() != 0 {
		t.Fatalf("denied terminal=%T effects=%d", terminal, bridge.count())
	}
	invocation.Member.Permissions = []contract.MemberPermission{contract.PermissionManageRoles}
	terminal, err = host.Run(context.Background(), "manager", invocation)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := terminal.(*contract.EditResponseOperation); !ok {
		t.Fatalf("terminal=%T", terminal)
	}
	if bridge.count() != 1 {
		t.Fatalf("effects=%d", bridge.count())
	}
}

func TestEffectExecutorRechecksCapabilitiesAtFinalBoundary(t *testing.T) {
	bridge := &recordingDiscordBridge{}
	host := &Host{hostServices: hostServices{bridge: Bridge{Discord: bridge}}}
	plugin := &Plugin{PluginSource: PluginSource{ID: "test"}}
	operation := &contract.SendDMOperation{UserID: "175928847299117063", Message: contract.Message{Content: "hello"}}
	invocation := contract.Invocation{InvocationActorContext: contract.InvocationActorContext{Author: &contract.UserRef{ID: "175928847299117064"}}}
	if err := host.executeEffect(context.Background(), plugin, invocation, operation); err == nil {
		t.Fatal("expected capability denial")
	}
	if bridge.count() != 0 {
		t.Fatal("denied effect reached bridge")
	}
	plugin.Capabilities = []contract.Capability{contract.CapabilityDiscordMessages}
	if err := host.executeEffect(context.Background(), plugin, invocation, operation); err != nil {
		t.Fatal(err)
	}
	if bridge.count() != 1 {
		t.Fatalf("effects=%d", bridge.count())
	}
}

func TestDecodeContractJSONRejectsInvalidTrailingData(t *testing.T) {
	for _, raw := range []string{"1 2", "1 ]", "{} trailing"} {
		if _, err := decodeContractJSON(raw); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
	if value, err := decodeContractJSON(`{"a":[1,true,null]}`); err != nil || value.Kind() != contract.ValueObject {
		t.Fatalf("value=%#v err=%v", value, err)
	}
}

func TestAdmissionRunsOnceAfterAuthorization(t *testing.T) {
	root := t.TempDir()
	writeTestPluginRoot(t, filepath.Join(root, "plain"), "plain")
	host, err := NewHost(Options{BundleOptions: BundleOptions{Dirs: []string{root}}, AuthorityOptions: AuthorityOptions{AllowUnsignedPlugin: true}, RuntimeOptions: RuntimeOptions{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}})
	if err != nil {
		t.Fatal(err)
	}
	if err = host.LoadAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	plan, err := host.PlanCommand("slash", "test", []string{"test"})
	if err != nil {
		t.Fatal(err)
	}
	invocation := contract.Invocation{InvocationIdentity: contract.InvocationIdentity{Route: plan.Route, Kind: contract.InvocationCommand}, InvocationInteractionContext: contract.InvocationInteractionContext{ResponseState: contract.ResponseUnacknowledged}, InvocationActorContext: contract.InvocationActorContext{Author: &contract.UserRef{ID: "175928847299117063"}}, InvocationInput: contract.InvocationInput{Command: &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"test"}}}}
	admission, denial, err := host.Admit(context.Background(), "plain", invocation)
	if err != nil || admission == nil || denial != nil {
		t.Fatalf("admission=%v denial=%#v err=%v", admission, denial, err)
	}
	terminal, err := admission.Run(context.Background(), contract.ResponseUnacknowledged)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := terminal.(*contract.MessageOperation); !ok {
		t.Fatalf("terminal=%T", terminal)
	}
	if _, err = admission.Run(context.Background(), contract.ResponseUnacknowledged); err == nil {
		t.Fatal("reused admission")
	}
}

func TestStaticPermissionDenialPrecedesCustomChecks(t *testing.T) {
	dir := t.TempDir()
	writeCustomCheckPlugin(t, dir)
	host, err := NewHost(Options{BundleOptions: BundleOptions{Dirs: []string{dir}}, AuthorityOptions: AuthorityOptions{AllowUnsignedPlugin: true}, RuntimeOptions: RuntimeOptions{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}})
	if err != nil {
		t.Fatal(err)
	}
	if err = host.LoadAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	plan, err := host.PlanCommand("slash", "protected", []string{"protected"})
	if err != nil {
		t.Fatal(err)
	}
	invocation := contract.Invocation{InvocationIdentity: contract.InvocationIdentity{Route: plan.Route, Kind: contract.InvocationCommand}, InvocationInteractionContext: contract.InvocationInteractionContext{ResponseState: contract.ResponseUnacknowledged}, InvocationActorContext: contract.InvocationActorContext{Guild: &contract.GuildRef{ID: "175928847299117063"}, Author: &contract.UserRef{ID: "175928847299117064"}, Member: &contract.MemberRef{GuildID: "175928847299117063", User: contract.UserRef{ID: "175928847299117064"}}}, InvocationInput: contract.InvocationInput{Command: &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"protected"}}}}
	admission, denial, err := host.Admit(context.Background(), "checks", invocation)
	if err != nil {
		t.Fatal(err)
	}
	if admission != nil || denial != nil {
		t.Fatalf("static denial admission=%v denial=%#v", admission, denial)
	}
	invocation.Member.Permissions = []contract.MemberPermission{contract.PermissionManageRoles}
	admission, denial, err = host.Admit(context.Background(), "checks", invocation)
	if err != nil {
		t.Fatal(err)
	}
	if admission != nil {
		t.Fatal("custom denial admitted")
	}
	if _, ok := denial.(*contract.MessageOperation); !ok {
		t.Fatalf("custom denial=%T", denial)
	}
}

func TestAdmissionReauthorizesAgainstReplacementGeneration(t *testing.T) {
	root := t.TempDir()
	pluginRoot := filepath.Join(root, "plain")
	writeTestPluginRoot(t, pluginRoot, "plain")
	host, err := NewHost(Options{BundleOptions: BundleOptions{Dirs: []string{root}}, AuthorityOptions: AuthorityOptions{AllowUnsignedPlugin: true}, RuntimeOptions: RuntimeOptions{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}})
	if err != nil {
		t.Fatal(err)
	}
	if err = host.LoadAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	plan, err := host.PlanCommand("slash", "test", []string{"test"})
	if err != nil {
		t.Fatal(err)
	}
	invocation := contract.Invocation{InvocationIdentity: contract.InvocationIdentity{Route: plan.Route, Kind: contract.InvocationCommand}, InvocationInteractionContext: contract.InvocationInteractionContext{ResponseState: contract.ResponseUnacknowledged}, InvocationActorContext: contract.InvocationActorContext{Author: &contract.UserRef{ID: "175928847299117063"}}, InvocationInput: contract.InvocationInput{Command: &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"test"}}}}
	admission, denial, err := host.Admit(context.Background(), "plain", invocation)
	if err != nil || admission == nil || denial != nil {
		t.Fatalf("admit=%v denial=%#v err=%v", admission, denial, err)
	}
	bundleDir := filepath.Join(pluginRoot, "bundles", "test-v0.1.0")
	source := `load("@mamacord//api.star", "cog", "plugin", "reply", "slash_command")
def run(ctx): return [reply(content="replacement")]
def setup(bot): bot.add_cog(cog(name="Test", commands=[slash_command(name="test", description="Test command", handler=run)]))
PLUGIN = plugin(setup=setup)
`
	if err = os.WriteFile(filepath.Join(bundleDir, "plugin.star"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if err = host.LoadAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	terminal, err := admission.Run(context.Background(), contract.ResponseUnacknowledged)
	if err != nil {
		t.Fatal(err)
	}
	message, ok := terminal.(*contract.MessageOperation)
	if !ok || message.Message.Content != "replacement" {
		t.Fatalf("terminal=%#v", terminal)
	}
}
