package models

import (
	"grpc/pkg/proto"

	"github.com/google/uuid"
)

type Session struct {
	ID       uuid.UUID
	GameID   uuid.UUID
	Username string
	events   chan<- *proto.GameEvent
}

func NewSession(gameID uuid.UUID, username string, events chan<- *proto.GameEvent) *Session {
	return &Session{
		ID:       uuid.New(),
		GameID:   gameID,
		Username: username,
		events:   events,
	}
}

func (s *Session) SendNonBlocking(event *proto.GameEvent) bool {
	select {
	case s.events <- event:
		return true
	default:
		return false
	}
}
