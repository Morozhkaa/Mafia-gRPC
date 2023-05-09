package main

import (
	"context"
	"grpc/internal/config"
	"grpc/internal/server"
	"grpc/pkg/infra/logger"
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

	// create listener and server engine
	lis, err := net.Listen("tcp", "127.0.0.1:"+cfg.HTTP_port)
	if err != nil {
		l.Sugar().Fatalf("listen failed: %v\n", err)
	}
	core, err := server.NewCore(ctx, server.DefaultConfig)
	if err != nil {
		l.Sugar().Fatalf("failed to create engine: %v\nconfig: %v\n", err, server.DefaultConfig)
	}

	// create server and start work
	srv := grpc.NewServer()
	proto.RegisterMafiaServer(srv, server.NewServer(ctx, core))
	go func() {
		err := srv.Serve(lis)
		if err != nil {
			l.Sugar().Fatalf("server failed: %v\n", err)
		}
	}()
	l.Info("server started on 127.0.0.1:" + cfg.HTTP_port)

	// when the context completes, exit
	<-ctx.Done()
	cancel()
}
