package main

import (
	"context"
	"fmt"
	"grpc/internal/client"
	"grpc/internal/config"
	"grpc/pkg/infra/logger"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const DefaultMaxRetries = 5
const DefaultRetryTimeout = 3 * time.Second

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer cancel()

	// get environment variable values ​​(HTTP_PORT, IsProd)
	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatalf("getting config failed: %s", err.Error())
	}

	// initialize logger
	optsLogger := logger.LoggerOptions{IsProd: cfg.IsProd}
	l, err := logger.New(optsLogger)
	if err != nil {
		log.Fatalf("logger initialization failed: %s", err.Error())
	}

	// create connection
	conn, err := grpc.Dial(
		"127.0.0.1:"+cfg.HTTP_port,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		l.Sugar().Fatalf("failed to connect: %v\n", err)
	}
	defer conn.Close()

	// get username
	fmt.Printf("Enter your username: ")
	var username string
	fmt.Scanf("%s", &username)

	// create client and start work
	cli, err := client.NewClient(ctx, username, conn)
	if err != nil {
		l.Sugar().Fatalf("failed to init grpc client: %v\n", err)
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
