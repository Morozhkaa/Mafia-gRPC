package main

import (
	"context"
	"fmt"
	"grpc/internal/config"
	"grpc/internal/server"
	"grpc/internal/server/domain/models"
	"grpc/pkg/proto"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer cancel()

	// get environment variable values ​​(HTTP_PORT)
	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatalf("getting config failed: %s", err.Error())
	}
	// create listener and server engine
	lis, err := net.Listen("tcp", "127.0.0.1:"+cfg.HTTP_port)
	if err != nil {
		log.Fatalf("listen failed: %v\n", err)
	}

	// create config with distribution of roles
	var config server.Config
	var s string
	fmt.Printf("Default: 4 players (Mafia, Commissar, 2 Innocents)\nWant to change the number of players? (y/n): ")
	fmt.Scanf("%s", &s)
	for {
		switch {
		case s == "y":
			fmt.Printf("Enter the number of players for the roles of the Mafia, Commissar, Innocent, separated by a space: ")
			var m, c, i uint
			fmt.Scan(&m, &c, &i)
			config = server.Config{
				RoleDistribution: map[models.Role]uint{
					models.RoleMafia:     m,
					models.RoleCommissar: c,
					models.RoleInnocent:  i,
				},
			}
			goto next
		case s == "n":
			config = server.DefaultConfig
			goto next
		default:
			fmt.Printf("Try again: enter 'y' to set the number of players, or 'n' to use the default values: ")
			fmt.Scanf("%s", &s)
		}
	}
next:
	core, err := server.NewCore(ctx, config)
	if err != nil {
		log.Fatalf("failed to create core: %v\nconfig: %v\n", err, server.DefaultConfig)
	}
	// create server and start work
	srv := grpc.NewServer()
	proto.RegisterMafiaServer(srv, server.NewServer(ctx, core))
	go func() {
		err := srv.Serve(lis)
		if err != nil {
			log.Fatalf("server failed: %v\n", err)
		}
	}()
	log.Print("server started on 127.0.0.1:" + cfg.HTTP_port)

	// when the context completes, exit
	<-ctx.Done()
	cancel()
}
