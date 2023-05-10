package models

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type Game struct {
	id            uuid.UUID
	dayNumber     uint
	dayPhase      bool // true when the current phase is day
	mu            sync.RWMutex
	players       []*Player
	kickVotes     map[uint]map[string]string
	killVotes     map[uint]map[string]string
	winners       *Team
	mafiaMap      map[*Player]struct{}
	commissarMap  map[*Player]struct{}
	kickMap       map[*Player]struct{}
	mafiaChan     chan struct{}
	commissarChan chan struct{}
	kickChan      chan struct{}
}

func NewGame() *Game {
	return &Game{
		id:            uuid.New(),
		kickVotes:     make(map[uint]map[string]string, 1),
		killVotes:     make(map[uint]map[string]string, 1),
		kickChan:      make(chan struct{}),
		mafiaChan:     make(chan struct{}),
		commissarChan: make(chan struct{}),
		kickMap:       make(map[*Player]struct{}),
		mafiaMap:      make(map[*Player]struct{}),
		commissarMap:  make(map[*Player]struct{}),
	}
}

// Declare getters
func (g *Game) ID() uuid.UUID {
	return g.id
}

func (g *Game) DayNumber() uint {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.dayNumber
}

func (g *Game) IsDayPhase() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.dayPhase
}

func (g *Game) Players() []*Player {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.players
}

func (g *Game) Winners() *Team {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.winners
}

func (g *Game) KickChan() chan struct{} {
	return g.commissarChan
}

func (g *Game) KickMap() map[*Player]struct{} {
	return g.commissarMap
}
func (g *Game) KickMapReopen() {
	g.commissarMap = make(map[*Player]struct{})
}

func (g *Game) CommissarChan() chan struct{} {
	return g.commissarChan
}

func (g *Game) CommissarMap() map[*Player]struct{} {
	return g.commissarMap
}
func (g *Game) CommissarMapReopen() {
	g.commissarMap = make(map[*Player]struct{})
}

func (g *Game) MafiaChan() chan struct{} {
	return g.mafiaChan
}

func (g *Game) MafiaMap() map[*Player]struct{} {
	return g.mafiaMap
}

func (g *Game) MafiaMapReopen() {
	g.mafiaMap = make(map[*Player]struct{})
}

// AddPlayer adds a new player to the game if its name is unique, otherwise returns an error.
func (g *Game) AddPlayer(p *Player) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, other := range g.players {
		if strings.EqualFold(other.Username(), p.Username()) {
			return errors.New("this username is already in use by one of the players")
		}
	}
	g.players = append(g.players, p)
	return nil
}

// addVote adds a record of who each member voted for on a given day.
func (g *Game) addVote(m map[uint]map[string]string, dayNumber uint, player, target *Player) {
	votes, exists := m[dayNumber]
	if !exists {
		votes = make(map[string]string, 1)
		m[dayNumber] = votes
	}
	votes[player.Username()] = target.Username()
}

// AddVote adds a record of who each players voted for on a given day.
func (g *Game) AddVote(dayNumber uint, player, target *Player) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.addVote(g.kickVotes, dayNumber, player, target)
	log.Printf("%s %s voted to kick %s\n", g, player.Username(), target.Username())
}

// AddKillVote adds a record of who mafia members wanted to kill on a specific night.
func (g *Game) AddKillVote(dayNumber uint, player, target *Player) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.addVote(g.killVotes, dayNumber, player, target)
	log.Printf("%s %s voted to kill %s\n", g, player.Username(), target.Username())
}

// findMostVoted returns the most voted victim. If it's not the only one, return nil.
func (g *Game) findMostVoted(votes map[string]string) *Player {
	cnt := make(map[string]uint)
	for _, candidate := range votes {
		cnt[candidate]++
	}
	victim, topCount, isAbsolute := "", uint(0), false
	for candidate, c := range cnt {
		log.Printf("%s %s has %d votes\n", g, candidate, c)
		if c == topCount {
			isAbsolute = false
		}
		if c > topCount {
			victim = candidate
			topCount = c
			isAbsolute = true
		}
	}
	if !isAbsolute {
		return nil
	}
	for _, p := range g.players {
		if strings.EqualFold(p.Username(), victim) {
			return p
		}
	}
	return nil
}

// NewDay starts a new day, returns the player who was killed last night and the next day number.
func (g *Game) NewDay() (*Player, uint) {
	g.mu.Lock()
	defer g.mu.Unlock()
	victim := g.findMostVoted(g.killVotes[g.dayNumber])
	if g.dayNumber == 0 {
		g.dayNumber++
		g.dayPhase = true
		return nil, g.dayNumber
	}
	if victim == nil {
		log.Printf("%s more than one player has received the most votes, a revote is required\n", g)
		return nil, 0
	}
	victim.Kill()
	g.dayNumber++
	g.dayPhase = true
	return victim, g.dayNumber
}

// NewNight starts a night and returns who was kicked out that day.
func (g *Game) NewNight() *Player {
	g.mu.Lock()
	defer g.mu.Unlock()
	kicked := g.findMostVoted(g.kickVotes[g.dayNumber])
	if kicked == nil {
		log.Printf("%s more than one player has received the most votes, a revote is required\n", g)
		return nil
	}
	kicked.Kill()
	g.dayPhase = false
	return kicked
}

// CheckStatus keeps track of game statuses and sets the winners if the game is over.
func (g *Game) CheckStatus() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.winners != nil {
		return
	}
	mafia, others := 0, 0
	for _, p := range g.players {
		if !p.Alive() {
			continue
		}
		switch p.Role() {
		case RoleInnocent, RoleCommissar:
			others++
		case RoleMafia:
			mafia++
		}
	}
	var winners Team
	if others <= mafia {
		winners = TeamMafia
		g.winners = &winners
	} else if mafia == 0 {
		winners = TeamCivilians
		g.winners = &winners
	}
}

// FindPlayer searches and returns a player by username.
func (g *Game) FindPlayer(username string) (*Player, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, p := range g.players {
		if strings.EqualFold(p.Username(), username) {
			return p, nil
		}
	}
	return nil, errors.New("player not found")
}

// FindMessageReceivers returns a list of players to send a message to.
func (g *Game) FindMessageReceivers(sender *Player) (receivers []*Player) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, p := range g.players {
		if strings.EqualFold(sender.Username(), p.Username()) {
			continue
		}
		var ok bool
		ok = ok || g.dayNumber == 0
		ok = ok || g.winners != nil
		ok = ok || !p.Alive()
		ok = ok || (g.dayPhase && sender.Alive())
		ok = ok || (!g.dayPhase && sender.Alive() && sender.Role() == RoleMafia && p.Role() == RoleMafia)
		if ok {
			receivers = append(receivers, p)
		}
	}
	return receivers
}

// Displaying the first digits of the game ID.
func (g *Game) String() string {
	return fmt.Sprintf("[#%s]", g.id.String()[:5])
}
