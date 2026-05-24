package catalog

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/disgoorg/disgo/discord"

	"github.com/xsyetopz/go-mamacord/internal/commands"
	"github.com/xsyetopz/go-mamacord/internal/i18n"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/slashcmd"
)

func testBuiltinCommands() []slashcmd.Command {
	out := []slashcmd.Command{}
	for _, module := range commands.Catalog() {
		out = append(out, BuiltinCommands(module)...)
	}
	return out
}

func TestCommandCreatesIncludesAllBuiltinCommands(t *testing.T) {
	t.Parallel()

	registry, err := i18n.LoadCore(filepath.Join("..", "..", "..", "..", "locales"))
	if err != nil {
		t.Fatalf("LoadCore: %v", err)
	}

	creates := CommandCreates(CommandCreateOptions{
		Builtins: testBuiltinCommands(),
		I18n:     registry,
		Locales:  registry.SupportedLocales(),
	})

	var names []string
	for _, create := range creates {
		switch cmd := create.(type) {
		case discord.SlashCommandCreate:
			names = append(names, cmd.Name)
		default:
			t.Fatalf("unexpected create type %T", create)
		}
	}
	slices.Sort(names)

	want := []string{"block", "help", "modules", "ping", "plugins", "unblock"}
	if !slices.Equal(names, want) {
		t.Fatalf("create names = %#v, want %#v", names, want)
	}
}

func TestCommandCreatesBuildsExpectedAdminShapes(t *testing.T) {
	t.Parallel()

	registry, err := i18n.LoadCore(filepath.Join("..", "..", "..", "..", "locales"))
	if err != nil {
		t.Fatalf("LoadCore: %v", err)
	}

	creates := CommandCreates(CommandCreateOptions{
		Builtins: testBuiltinCommands(),
		I18n:     registry,
		Locales:  registry.SupportedLocales(),
	})

	byName := map[string]discord.SlashCommandCreate{}
	for _, create := range creates {
		cmd, ok := create.(discord.SlashCommandCreate)
		if !ok {
			t.Fatalf("unexpected create type %T", create)
		}
		byName[cmd.Name] = cmd
	}

	block := byName["block"]
	if len(block.Options) != 2 {
		t.Fatalf("block options = %d, want 2", len(block.Options))
	}

	modules := byName["modules"]
	if len(modules.Options) != 6 {
		t.Fatalf("modules options = %d, want 6", len(modules.Options))
	}

	plugins := byName["plugins"]
	if len(plugins.Options) != 8 {
		t.Fatalf("plugins options = %d, want 8", len(plugins.Options))
	}
}
