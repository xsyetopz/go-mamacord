package appcmd

import (
	"context"
	"io"
	"log/slog"
	"reflect"
	"testing"

	"github.com/disgoorg/disgo/discord"

	commandruntime "github.com/xsyetopz/go-mamacord/internal/commandruntime"
	"github.com/xsyetopz/go-mamacord/internal/marketplace"
	moduleapi "github.com/xsyetopz/go-mamacord/internal/modules"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
)

type fakePluginAdmin struct{}

func (fakePluginAdmin) Configured() bool               { return true }
func (fakePluginAdmin) Infos() []pluginhost.PluginInfo { return nil }
func (fakePluginAdmin) Reload(context.Context) error   { return nil }

type fakeModuleAdmin struct{}

func (fakeModuleAdmin) Configured() bool                                       { return true }
func (fakeModuleAdmin) Infos() []moduleapi.Info                                { return nil }
func (fakeModuleAdmin) Reload(context.Context) error                           { return nil }
func (fakeModuleAdmin) SetEnabled(context.Context, string, bool, uint64) error { return nil }
func (fakeModuleAdmin) Reset(context.Context, string) error                    { return nil }

type fakeMarketplaceAdmin struct{}

func (fakeMarketplaceAdmin) Configured() bool { return true }
func (fakeMarketplaceAdmin) ListSources(context.Context) ([]marketplace.Source, error) {
	return nil, nil
}
func (fakeMarketplaceAdmin) UpsertSource(context.Context, marketplace.SourceUpsert) (marketplace.Source, error) {
	return marketplace.Source{}, nil
}
func (fakeMarketplaceAdmin) DeleteSource(context.Context, string) error { return nil }
func (fakeMarketplaceAdmin) SyncSource(context.Context, string) (marketplace.SyncResult, error) {
	return marketplace.SyncResult{}, nil
}
func (fakeMarketplaceAdmin) Search(context.Context, marketplace.SearchQuery) ([]marketplace.PluginCandidate, error) {
	return nil, nil
}
func (fakeMarketplaceAdmin) Install(context.Context, marketplace.InstallRequest) (marketplace.InstallResult, error) {
	return marketplace.InstallResult{}, nil
}
func (fakeMarketplaceAdmin) Update(context.Context, marketplace.UpdateRequest) (marketplace.UpdateResult, error) {
	return marketplace.UpdateResult{}, nil
}
func (fakeMarketplaceAdmin) Uninstall(context.Context, marketplace.UninstallRequest) error {
	return nil
}
func (fakeMarketplaceAdmin) TrustSigner(context.Context, marketplace.TrustSignerRequest) error {
	return nil
}
func (fakeMarketplaceAdmin) TrustVendor(context.Context, marketplace.TrustVendorRequest) (marketplace.TrustVendorResult, error) {
	return marketplace.TrustVendorResult{}, nil
}

func TestRuntimeServicesReturnsInjectedDependencies(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	plugins := fakePluginAdmin{}
	modules := fakeModuleAdmin{}
	marketplaceAdmin := fakeMarketplaceAdmin{}

	var helpCalled bool
	r := Runtime{
		Logger:      logger,
		ProdMode:    true,
		Plugins:     plugins,
		Modules:     modules,
		Marketplace: marketplaceAdmin,
		IsOwner: func(userID uint64) bool {
			return userID == 7
		},
		HelpNames: func(locale string) []string {
			helpCalled = locale == discord.LocaleEnglishUS.Code()
			return []string{"ping", "help"}
		},
	}

	s := r.Services(discord.LocaleEnglishUS)

	if s.Logger != logger {
		t.Fatalf("Services.Logger mismatch")
	}
	if !s.ProdMode {
		t.Fatalf("Services.ProdMode = false, want true")
	}
	if s.IsOwner == nil || !s.IsOwner(7) || s.IsOwner(8) {
		t.Fatalf("Services.IsOwner mismatch")
	}
	if s.Plugins == nil || !s.Plugins.Configured() {
		t.Fatalf("Services.Plugins not preserved")
	}
	if s.Modules == nil || !s.Modules.Configured() {
		t.Fatalf("Services.Modules not preserved")
	}
	if s.Marketplace == nil || !s.Marketplace.Configured() {
		t.Fatalf("Services.Marketplace not preserved")
	}
	if got := s.HelpNames(discord.LocaleEnglishUS.Code()); !reflect.DeepEqual(got, []string{"ping", "help"}) {
		t.Fatalf("Services.HelpNames() = %#v, want ping/help", got)
	}
	if !helpCalled {
		t.Fatalf("Services.HelpNames did not call injected function")
	}
}

var _ commandruntime.PluginAdmin = fakePluginAdmin{}
var _ moduleapi.Admin = fakeModuleAdmin{}
var _ commandruntime.MarketplaceAdmin = fakeMarketplaceAdmin{}
