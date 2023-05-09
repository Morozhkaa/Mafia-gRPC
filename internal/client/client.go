package client

import (
	"context"
	"grpc/pkg/proto"
	"log"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type Client struct {
	ctx    context.Context
	cli    proto.MafiaClient
	stream proto.Mafia_JoinGameClient
	events chan *proto.GameEvent
}

func NewClient(ctx context.Context, username string, conn *grpc.ClientConn) (*Client, error) {
	cli := proto.NewMafiaClient(conn)
	stream, err := cli.JoinGame(ctx, &proto.JoinGameRequest{Username: username})
	if err != nil {
		return nil, errors.Wrap(err, "failed to join a game")
	}
	md, err := stream.Header()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get gRPC headers")
	}
	sessionID, err := proto.FetchSessionID(md)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get session ID")
	}
	ctx = metadata.NewOutgoingContext(ctx, proto.WithSessionID(sessionID))
	return &Client{
		ctx:    ctx,
		cli:    cli,
		stream: stream,
		events: make(chan *proto.GameEvent),
	}, nil
}

func (c *Client) Events() <-chan *proto.GameEvent {
	return c.events
}

func (c *Client) ForwardEvents() {
	for {
		event, err := c.stream.Recv()
		if err != nil {
			log.Fatalf("\n\nServer closed\n")
			return
		}
		c.events <- event
	}
}

func (c *Client) SendMessage(content string) (receivers uint, err error) {
	resp, err := c.cli.SendMessage(c.ctx, &proto.SendMessageRequest{Content: content})
	if err != nil {
		return 0, err
	}
	return uint(resp.GetReceiverCount()), nil
}

func (c *Client) GetGameState() (*proto.GetGameStatusResponse, error) {
	return c.cli.GetGameStatus(c.ctx, &proto.GetGameStatusRequest{})
}

func (c *Client) VoteKick(username string) error {
	_, err := c.cli.Kick(c.ctx, &proto.KickRequest{Username: username})
	return err
}

func (c *Client) VoteKill(username string) error {
	_, err := c.cli.Kill(c.ctx, &proto.KillRequest{Username: username})
	return err
}
