package server

import (
	"context"
	"grpc/internal/server/domain/models"
	"grpc/pkg/proto"
	"log"
	"strings"

	"github.com/pkg/errors"
	"google.golang.org/grpc/metadata"
)

const EventsBufferSize = 8

type Server struct {
	proto.UnimplementedMafiaServer
	ctx  context.Context
	core *Core
}

func NewServer(ctx context.Context, core *Core) *Server {
	return &Server{
		ctx:  ctx,
		core: core,
	}
}

func (s *Server) JoinGame(in *proto.JoinGameRequest, stream proto.Mafia_JoinGameServer) error {
	if in.GetUsername() == "" {
		return errors.New("empty username")
	}
	events := make(chan *proto.GameEvent, EventsBufferSize)
	sess, err := s.core.AddPlayer(in.GetUsername(), events)
	if err != nil {
		return err
	}
	err = stream.SendHeader(proto.WithSessionID(sess.ID))
	if err != nil {
		log.Printf("failed to send stream header: %v\n", err)
		return errors.New("failed to send metadata header")
	}
	for {
		var event *proto.GameEvent
		select {
		case <-s.ctx.Done():
			return nil
		case event = <-events:
		}
		err := stream.Send(event)
		if err != nil {
			log.Printf("failed to send an event: %v\n%v\n", err, event)
			s.core.RemovePlayer(sess)
			break
		}
	}
	return nil
}

func (s *Server) fetchPlayer(ctx context.Context) (*models.Player, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("missed metadata")
	}
	sessionID, err := proto.FetchSessionID(md)
	if err != nil {
		return nil, err
	}
	p, err := s.core.FindPlayerBySessionID(sessionID)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Server) SendMessage(ctx context.Context, in *proto.SendMessageRequest) (*proto.SendMessageResponse, error) {
	p, err := s.fetchPlayer(ctx)
	if err != nil {
		return nil, err
	}
	if in.GetContent() == "" {
		return nil, errors.New("missed content")
	}
	receivers, err := s.core.SendMessage(p, in.GetContent())
	if err != nil {
		return nil, err
	}
	return &proto.SendMessageResponse{
		ReceiverCount: uint64(len(receivers)),
	}, nil
}

func (s *Server) Kick(ctx context.Context, in *proto.KickRequest) (*proto.KickResponse, error) {
	p, err := s.fetchPlayer(ctx)
	if err != nil {
		return nil, err
	}
	if in.GetUsername() == "" {
		return nil, errors.New("missed username")
	}
	err = s.core.KickVote(p, in.GetUsername())
	if err != nil {
		return nil, err
	}
	return &proto.KickResponse{}, nil
}

func (s *Server) Kill(ctx context.Context, in *proto.KillRequest) (*proto.KillResponse, error) {
	p, err := s.fetchPlayer(ctx)
	if err != nil {
		return nil, err
	}
	if in.GetUsername() == "" {
		return nil, errors.New("missed username")
	}
	err = s.core.KillVote(p, in.GetUsername())
	if err != nil {
		return nil, err
	}
	return &proto.KillResponse{}, nil
}

func (s *Server) CheckUser(ctx context.Context, in *proto.CheckUserRequest) (*proto.CheckUserResponse, error) {
	p, err := s.fetchPlayer(ctx)
	if err != nil {
		return nil, err
	}
	if in.GetUsername() == "" {
		return nil, errors.New("missed username")
	}
	isMafia, err := s.core.CheckUser(p, in.GetUsername())
	if err != nil {
		return nil, err
	}
	return &proto.CheckUserResponse{IsMafia: isMafia}, nil
}

func (s *Server) GetGameStatus(ctx context.Context, in *proto.GetGameStatusRequest) (*proto.GetGameStatusResponse, error) {
	p, err := s.fetchPlayer(ctx)
	if err != nil {
		return nil, err
	}
	g, err := s.core.FindGameByID(p.Session().GameID)
	if err != nil {
		return nil, err
	}
	players := g.Players()
	response := &proto.GetGameStatusResponse{
		Self:    p.ConvertToProtoPlayer(),
		Players: make([]*proto.Player, 0, len(g.Players())),
	}
	role := p.Role().ConvertToProtoRole()
	response.Self.Role = &role

	if g.Winners() != nil {
		team := g.Winners().ConvertToProtoTeam()
		response.Winners = &team
	}
	for _, other := range players {
		showRole := g.Winners() != nil
		showRole = showRole || !p.Alive()
		showRole = showRole || strings.EqualFold(p.Username(), other.Username())
		showRole = showRole || (p.Role() == models.RoleMafia && other.Role() == models.RoleMafia)

		converted := other.ConvertToProtoPlayer()
		if showRole {
			role := other.Role().ConvertToProtoRole()
			converted.Role = &role
		}

		response.Players = append(response.Players, converted)
	}
	return response, nil
}
