package dbot

import (
	"fmt"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

func Error(e *events.ApplicationCommandInteractionEvent, content string) error {
	return e.CreateMessage(discord.MessageCreate{Content: content, Flags: discord.MessageFlagEphemeral})
}

func Errorf(e *events.ApplicationCommandInteractionEvent, str string, a ...any) error {
	return Error(e, fmt.Sprintf(str, a))
}

func Success(e *events.ApplicationCommandInteractionEvent, content string) error {
	return e.CreateMessage(discord.MessageCreate{Content: content})
}

func Successf(e *events.ApplicationCommandInteractionEvent, str string, a ...any) error {
	return Success(e, fmt.Sprintf(str, a))
}
