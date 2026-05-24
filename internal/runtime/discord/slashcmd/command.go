package slashcmd

import (
	"context"

	"github.com/disgoorg/disgo/events"

	commandruntime "github.com/xsyetopz/go-mamacord/internal/commandruntime"
	commandtext "github.com/xsyetopz/go-mamacord/internal/commandtext"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/interactions"
)

type Handler func(
	ctx context.Context,
	e *events.ApplicationCommandInteractionCreate,
	t commandtext.Translator,
	s commandruntime.Services,
) (interactions.SlashAction, error)

type Command struct {
	Name   string
	NameID string
	DescID string
	Handle Handler
}
