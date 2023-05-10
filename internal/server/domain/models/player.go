package models

import (
	"grpc/pkg/proto"
	"sync"
)

type Role string

const (
	RoleMafia     Role = "MAFIA"
	RoleCommissar Role = "COMMISSAR"
	RoleInnocent  Role = "INNOCENT"
)

// ConvertToProtoRole returns the corresponding proto.Role instance.
func (r Role) ConvertToProtoRole() proto.Role {
	switch r {
	case RoleMafia:
		return proto.Role_ROLE_MAFIOSI
	case RoleCommissar:
		return proto.Role_ROLE_COMMISSAR
	case RoleInnocent:
		return proto.Role_ROLE_INNOCENT
	default:
		return proto.Role_ROLE_UNKNOWN
	}
}

type Player struct {
	username string
	alive    bool
	role     Role
	session  *Session
	mu       sync.RWMutex
}

func NewPlayer(name string, role Role, s *Session) *Player {
	return &Player{
		username: name,
		alive:    true,
		role:     role,
		session:  s,
	}
}

// Declare getters
func (p *Player) Username() string {
	return p.username
}

func (p *Player) Alive() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.alive
}

func (p *Player) Role() Role {
	return p.role
}

func (p *Player) Session() *Session {
	return p.session
}

// Kill changes the player's state to not alive.
func (p *Player) Kill() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.alive = false
}

// ConvertToProtoPlayer returns the corresponding proto.Player instance.
func (p *Player) ConvertToProtoPlayer() *proto.Player {
	if p == nil {
		return nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return &proto.Player{
		Username: p.username,
		Alive:    p.alive,
	}
}

type Team string

const (
	TeamCivilians Team = "CIVILIANS"
	TeamMafia     Team = "MAFIA"
)

// ConvertToProtoTeam returns the corresponding proto.Team instance.
func (t Team) ConvertToProtoTeam() proto.Team {
	switch t {
	case TeamCivilians:
		return proto.Team_TEAM_CIVILIANS
	case TeamMafia:
		return proto.Team_TEAM_MAFIA
	default:
		return proto.Team_TEAM_UNKNOWN
	}
}
