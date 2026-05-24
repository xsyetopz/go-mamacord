package modules

import "context"

type Kind string

const (
	KindCoreBuiltin Kind = "core_builtin"
	KindPlugin      Kind = "plugin"
)

type Runtime string

const (
	RuntimeGo  Runtime = "go"
	RuntimeLua Runtime = "lua"
)

type Info struct {
	ID             string
	Name           string
	Kind           Kind
	Runtime        Runtime
	Enabled        bool
	DefaultEnabled bool
	Toggleable     bool
	Signed         bool
	Source         string
	Commands       []string
}

type Admin interface {
	Configured() bool
	Infos() []Info
	Reload(ctx context.Context) error
	SetEnabled(ctx context.Context, moduleID string, enabled bool, actorID uint64) error
	Reset(ctx context.Context, moduleID string) error
}
