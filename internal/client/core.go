package client

import (
	"context"
	"fmt"
	"grpc/pkg/proto"
	"strings"
)

type Core struct {
	ctx       context.Context
	client    *Client
	messenger *MessageReceiver
}

func NewCore(ctx context.Context, client *Client, messenger *MessageReceiver) *Core {
	return &Core{
		ctx:       ctx,
		client:    client,
		messenger: messenger,
	}
}

func (e *Core) Start() {
	go e.handleEvents()
	go e.handleUserInput()
	e.printCommandList(false)
	e.printStatus()
}

func (e *Core) handleEvents() {
	for {
		var event *proto.GameEvent
		select {
		case <-e.ctx.Done():
			return
		case event = <-e.client.Events():
		}

		switch event.GetType() {
		case proto.GameEvent_EVENT_PLAYER_JOINED:
			e.sendActionDescriptionWithComment(fmt.Sprintf("%s joined the game", event.GetPayloadPlayerJoined().GetPlayer().GetUsername()))
		case proto.GameEvent_EVENT_PLAYER_LEFT:
			e.sendActionDescriptionWithComment(fmt.Sprintf("%s left the game", event.GetPayloadPlayerLeft().GetPlayer().GetUsername()))

		case proto.GameEvent_EVENT_MESSAGE:
			e.sendWithComment(fmt.Sprintf("[%s]: %s", event.GetPayloadMessage().GetSender().GetUsername(), event.GetPayloadMessage().GetContent()))

		case proto.GameEvent_EVENT_DAY_STARTED:
			var text string
			if event.GetPayloadDayStarted().GetKilledPlayer() != nil {
				text = fmt.Sprintf("%s was murdered that night", event.GetPayloadDayStarted().GetKilledPlayer().GetUsername())
				e.sendActionDescription(text)
			}
			text = fmt.Sprintf("Day No. %d started", event.GetPayloadDayStarted().GetDayId())
			e.sendActionDescriptionWithComment(text)

		case proto.GameEvent_EVENT_NIGHT_STARTED:
			var text string
			if event.GetPayloadNightStarted().GetKickedPlayer() != nil {
				text = fmt.Sprintf("The majority voted to kick %s today", event.GetPayloadNightStarted().GetKickedPlayer().GetUsername())
				e.sendActionDescription(text)
			}
			text = fmt.Sprintf("Night No. %d started", event.GetPayloadNightStarted().GetDayId())
			e.sendActionDescriptionWithComment(text)

		case proto.GameEvent_EVENT_GAME_FINISHED:
			e.sendActionDescriptionWithComment(fmt.Sprintf("Game finished! Winners: %s", event.GetPayloadGameFinished().GetWinners().String()))

		default:
			e.sendWithComment(fmt.Sprintf("Received an event with type %s", event.GetType()))
		}
	}
}

func (e *Core) handleUserInput() {
	for {
		var input string
		select {
		case <-e.ctx.Done():
			return
		case input = <-e.messenger.InputChan():
		}

		switch {
		case strings.HasPrefix(input, "help"):
			e.printCommandList(true)

		case strings.HasPrefix(input, "status"):
			e.printStatus()

		case strings.HasPrefix(input, "msg"):
			e.printMsg(input)

		case strings.HasPrefix(input, "kick"):
			e.handleKickCommand(input)

		case strings.HasPrefix(input, "kill"):
			e.handleKillCommand(input)
		default:
			e.sendError("Invalid command, please see help.")
		}
	}
}

func (e *Core) printCommandList(withComment bool) {
	text := `===================================================================================================
Command list:

> help - print this message.
> status - print the current game status.
> msg [text] - send a text message.
> kick [username] - vote for kick someone (available during the day).
> kill [username] - vote for kill someone (available for mafiosi during the night).

===================================================================================================
`
	switch {
	case withComment:
		e.sendWithComment(text)
	default:
		e.messenger.OutputChan() <- "\n" + text
	}
}

func (e *Core) roleToString(r proto.Role) string {
	switch r {
	case proto.Role_ROLE_INNOCENT:
		return "[INNOCENT]"
	case proto.Role_ROLE_SHERIFF:
		return "[SHERIFF] "
	case proto.Role_ROLE_MAFIOSI:
		return "[MAFIOSI] "
	default:
		return "[]        "
	}
}

func (e *Core) printStatus() {
	state, err := e.client.GetGameState()
	if err != nil {
		e.sendError(err.Error())
		return
	}
	text := "=== GAME STATUS ========:\n"
	text += fmt.Sprintf("Your username: %s\n", state.GetSelf().GetUsername())
	text += "Players:"
	for _, p := range state.GetPlayers() {
		text += fmt.Sprintf("\n\t%s %s", e.roleToString(p.GetRole()), p.GetUsername())
		if !p.GetAlive() {
			text += "\t[x]"
		}
	}
	if state.GetWinners() != proto.Team_TEAM_UNKNOWN {
		text += "\n\nWinners: " + state.GetWinners().String()
	}
	text += "\n\n========================"
	e.sendWithComment(text)
}

func (e *Core) printMsg(input string) {
	content := input[len("msg "):]
	if content == "" {
		e.sendError("The message cannot be empty")
		return
	}
	receivers, err := e.client.SendMessage(content)
	if err != nil {
		e.sendError(fmt.Sprintf("Failed to send message: %v", err))
		return
	}
	e.sendActionDescriptionWithComment(fmt.Sprintf("The message has been sent to %d players", receivers))
}

func (e *Core) handleKickCommand(input string) {
	username := input[len("kick "):]
	err := e.client.VoteKick(username)
	if err != nil {
		e.sendError(err.Error())
		return
	}
	e.sendActionDescriptionWithComment(fmt.Sprintf("You cast your vote for %s", username))
}

func (e *Core) handleKillCommand(input string) {
	username := input[len("kill "):]
	err := e.client.VoteKill(username)
	if err != nil {
		e.sendError(err.Error())
		return
	}
	e.sendActionDescriptionWithComment(fmt.Sprintf("You cast your vote for %s", username))
}

func (e *Core) sendError(text string) {
	text = strings.ReplaceAll(text, "rpc error: code = Unknown desc =", "")
	e.sendWithComment("[ERROR] " + text)
}

func (e *Core) sendWithComment(text string) {
	e.messenger.OutputChan() <- "\n" + text + "\n\n" + "Enter command: "
}

func (e *Core) sendActionDescriptionWithComment(text string) {
	e.messenger.OutputChan() <- "\n\t\t\t\t\t*** " + text + " ***\n\n" + "Enter command: "
}

func (e *Core) sendActionDescription(text string) {
	e.messenger.OutputChan() <- "\n\t\t\t\t\t*** " + text + " ***\n\n"
}
