package main

import (
	"context"
	"fmt"
	"grpc/internal/client"
	"grpc/internal/config"
	"log"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer cancel()

	// get environment variable values ​​(HTTP_PORT)
	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatalf("getting config failed: %s", err.Error())
	}

	// create connection
	conn, err := grpc.Dial(
		"127.0.0.1:"+cfg.HTTP_port,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("failed to connect: %v\n", err)
	}
	defer conn.Close()

	// get username
	var cli *client.Client
	fmt.Printf("Enter your username: ")
	var username string
	fmt.Scanf("%s", &username)
	for {
		// create client and start work
		cli, err = client.NewClient(ctx, username, conn)
		if err == nil {
			break
		}
		fmt.Print("This username may have already been taken, please try again: ")
		fmt.Scanf("%s", &username)
	}

	messenger := client.NewMessenger(ctx)
	core := client.NewCore(ctx, cli, messenger)
	go messenger.Start()
	go core.Start()
	go cli.ForwardEvents()

	// when the context completes, exit
	<-ctx.Done()
	cancel()
}
