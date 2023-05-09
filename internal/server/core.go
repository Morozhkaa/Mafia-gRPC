package server

import (
	"context"
	"grpc/internal/server/domain/models"
	"grpc/pkg/proto"
	"log"
	"math/rand"
	"sync"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type Config struct {
	RoleDistribution map[models.Role]uint
}

var DefaultConfig = Config{
	RoleDistribution: map[models.Role]uint{
		models.RoleMafia:    1,
		models.RoleSheriff:  1,
		models.RoleInnocent: 2,
	},
}

type Core struct {
	ctx              context.Context
	config           Config
	sessionsLocker   sync.RWMutex
	sessions         map[uuid.UUID]*models.Session
	gamesLocker      sync.RWMutex
	games            map[uuid.UUID]*models.Game
	latestGameLocker sync.Mutex
	latestGame       *models.Game
	playersLeft      uint
	rolesToSelect    map[models.Role]uint
}

func NewCore(ctx context.Context, config Config) (*Core, error) {
	if config.RoleDistribution[models.RoleMafia] == 0 {
		return nil, errors.New("impossible to host games without mafiosi")
	}
	return &Core{
		ctx:      ctx,
		config:   config,
		sessions: make(map[uuid.UUID]*models.Session),
		games:    make(map[uuid.UUID]*models.Game),
	}, nil
}

func (e *Core) AddPlayer(username string, events chan<- *proto.GameEvent) (*models.Session, error) {
	e.latestGameLocker.Lock()
	defer e.latestGameLocker.Unlock()

	if e.latestGame == nil {
		e.hostNewGame()
	}

	// Select a role.
	var role models.Role

	rnd := uint(rand.Intn(int(e.playersLeft)))
	for r, cntLeft := range e.rolesToSelect {
		if rnd < cntLeft {
			role = r
			break
		}

		rnd -= cntLeft
	}

	sess := models.NewSession(e.latestGame.ID(), username, events)
	p := models.NewPlayer(username, role, sess)
	err := e.latestGame.AddPlayer(p)
	if err != nil {
		return nil, err
	}

	e.addSession(sess)

	e.rolesToSelect[role] = e.rolesToSelect[role] - 1
	e.playersLeft--

	defer e.broadcast(e.latestGame, &proto.GameEvent{
		Type: proto.GameEvent_EVENT_PLAYER_JOINED,
		Payload: &proto.GameEvent_PayloadPlayerJoined_{
			PayloadPlayerJoined: &proto.GameEvent_PayloadPlayerJoined{
				Player: p.ConvertToProtoPlayer(),
			},
		},
	})

	// If there are no free slots, start a game.
	if e.playersLeft == 0 {
		g := e.latestGame
		e.latestGame = nil

		go e.startGame(g)
	}

	return sess, err
}

func (e *Core) RemovePlayer(s *models.Session) {
	p, err := e.FindPlayerBySessionID(s.ID)
	if err != nil {
		return
	}

	g, err := e.FindGameByID(s.GameID)
	if err != nil {
		return
	}

	p.Kill()
	g.CheckStatus()

	e.broadcast(e.latestGame, &proto.GameEvent{
		Type: proto.GameEvent_EVENT_PLAYER_LEFT,
		Payload: &proto.GameEvent_PayloadPlayerLeft_{
			PayloadPlayerLeft: &proto.GameEvent_PayloadPlayerLeft{
				Player: p.ConvertToProtoPlayer(),
			},
		},
	})
}

func (e *Core) hostNewGame() {
	e.latestGame = models.NewGame()
	e.playersLeft = 0
	e.rolesToSelect = make(map[models.Role]uint)

	for role, cnt := range e.config.RoleDistribution {
		e.playersLeft += cnt
		e.rolesToSelect[role] = cnt
	}
	e.addGame(e.latestGame)
}

func (e *Core) broadcast(g *models.Game, event *proto.GameEvent) {
	for _, p := range g.Players() {
		p.Session().SendNonBlocking(event)
	}
}

func (e *Core) startGame(g *models.Game) {
	log.Printf("%s game stared\n", g)
	playersCnt := 4

	for {
		killedPlayer, dayID := g.NewDay()
		log.Printf("%s day %d started\n", g, dayID)
		if killedPlayer != nil {
			log.Printf("%s %s was killed\n", g, killedPlayer.Username())
			playersCnt--
		}

		e.broadcast(g, &proto.GameEvent{
			Type: proto.GameEvent_EVENT_DAY_STARTED,
			Payload: &proto.GameEvent_PayloadDayStarted_{
				PayloadDayStarted: &proto.GameEvent_PayloadDayStarted{
					DayId:        uint64(dayID),
					KilledPlayer: killedPlayer.ConvertToProtoPlayer(),
				},
			},
		})
		g.CheckStatus()
		if g.Winners() != nil {
			e.endGame(g)
			return
		}
		// Wait until all players cast their vote (enter the kick command)
		ch := g.DayChange()
		for i := 0; i < playersCnt; i++ {
			<-ch
		}

		kickedPlayer := g.NewNight()
		log.Printf("%s night %d started\n", g, dayID)
		if kickedPlayer != nil {
			log.Printf("%s %s was kicked\n", g, kickedPlayer.Username())
			playersCnt--
		}

		e.broadcast(g, &proto.GameEvent{
			Type: proto.GameEvent_EVENT_NIGHT_STARTED,
			Payload: &proto.GameEvent_PayloadNightStarted_{
				PayloadNightStarted: &proto.GameEvent_PayloadNightStarted{
					DayId:        uint64(dayID),
					KickedPlayer: kickedPlayer.ConvertToProtoPlayer(),
				},
			},
		})

		g.CheckStatus()
		if g.Winners() != nil {
			e.endGame(g)
			return
		}
		// Wait for the mafia to give its vote (enter the kill command)
		ch = g.DayChange()
		<-ch
	}
}

func (e *Core) endGame(g *models.Game) {
	log.Printf("%s game finished\n", g)
	e.broadcast(g, &proto.GameEvent{
		Type: proto.GameEvent_EVENT_GAME_FINISHED,
		Payload: &proto.GameEvent_PayloadGameFinished_{
			PayloadGameFinished: &proto.GameEvent_PayloadGameFinished{
				Winners: g.Winners().ConvertToProtoTeam(),
				Players: nil,
			},
		},
	})
}

func (e *Core) SendMessage(sender *models.Player, content string) ([]*models.Player, error) {
	g, err := e.FindGameByID(sender.Session().GameID)
	if err != nil {
		return nil, err
	}

	senderpb := sender.ConvertToProtoPlayer()

	candidates := g.FindMessageReceivers(sender)
	receivers := make([]*models.Player, 0, len(candidates))
	for _, p := range candidates {
		msg := &proto.GameEvent{
			Type: proto.GameEvent_EVENT_MESSAGE,
			Payload: &proto.GameEvent_PayloadMessage_{
				PayloadMessage: &proto.GameEvent_PayloadMessage{
					Sender:  senderpb,
					Content: content,
				},
			},
		}
		if p.Session().SendNonBlocking(msg) {
			receivers = append(receivers, p)
		}
	}

	return receivers, nil
}

func (e *Core) KickVote(voter *models.Player, candidate string) error {
	g, err := e.FindGameByID(voter.Session().GameID)
	if err != nil {
		return err
	}
	if g.DayNumber() == 0 {
		return errors.New("game not started")
	}
	if g.Winners() != nil {
		return errors.New("game finished")
	}
	if !voter.Alive() {
		return errors.New("you are dead")
	}
	if !g.IsDayPhase() {
		return errors.New("it's night now")
	}
	target, err := g.FindPlayer(candidate)
	if err != nil {
		return errors.New("invalid username")
	}
	g.AddVote(g.DayNumber(), voter, target)
	g.DayChange() <- struct{}{}
	return nil
}

func (e *Core) KillVote(voter *models.Player, candidate string) error {
	g, err := e.FindGameByID(voter.Session().GameID)
	if err != nil {
		return err
	}
	if g.DayNumber() == 0 {
		return errors.New("game not started")
	}
	if g.Winners() != nil {
		return errors.New("game finished")
	}
	if !voter.Alive() {
		return errors.New("you are dead")
	}
	if voter.Role() != models.RoleMafia {
		return errors.New("only mafiosi can kill")
	}
	if g.IsDayPhase() {
		return errors.New("it's day now")
	}
	target, err := g.FindPlayer(candidate)
	if err != nil {
		return errors.New("invalid username")
	}
	g.AddKillVote(g.DayNumber(), voter, target)
	g.DayChange() <- struct{}{}
	return nil
}

func (e *Core) FindPlayerBySessionID(id uuid.UUID) (*models.Player, error) {
	s, err := e.findSession(id)
	if err != nil {
		return nil, err
	}
	g, err := e.FindGameByID(s.GameID)
	if err != nil {
		return nil, err
	}
	p, err := g.FindPlayer(s.Username)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (e *Core) addSession(s *models.Session) {
	e.sessionsLocker.Lock()
	defer e.sessionsLocker.Unlock()
	e.sessions[s.ID] = s
}

func (e *Core) findSession(id uuid.UUID) (*models.Session, error) {
	e.sessionsLocker.RLock()
	defer e.sessionsLocker.RUnlock()
	s, exists := e.sessions[id]
	if !exists {
		return nil, errors.New("session not found")
	}
	return s, nil
}

func (e *Core) addGame(g *models.Game) {
	e.gamesLocker.Lock()
	defer e.gamesLocker.Unlock()
	e.games[g.ID()] = g
}

func (e *Core) FindGameByID(id uuid.UUID) (*models.Game, error) {
	e.gamesLocker.RLock()
	defer e.gamesLocker.RUnlock()
	g, exists := e.games[id]
	if !exists {
		return nil, errors.New("game not found")
	}
	return g, nil
}
