package app

import (
	"context"
	"log"
	"log/slog"
	"os"
	"time"

	pb "MrFood/services/auth/internal/api/grpc/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func (app *App) RunClient() {
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			slog.Error("failed to close grpc connection:", "error", err)
			os.Exit(1)
		}
	}()

	client := pb.NewTemplateServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	RegisterProcess(client, ctx)

	for range 10 {
		// Example 1: 1 Ping and 1 Pong
		time.Sleep(2 * time.Second)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		SinglePing(client, ctx)
		cancel()
	}
}

func SinglePing(client pb.TemplateServiceClient, ctx context.Context) {
	res, err := client.PingPong(ctx, &pb.Ping{Id: 1})
	slog.Info("Ping 1")
	if err != nil {
		slog.Error("failed", "error", err)
		os.Exit(1)
	}
	slog.Info("Pong", "id", res.Id)
}

func RegisterProcess(client pb.TemplateServiceClient, ctx context.Context) {
	res, err := client.RegisterProcess(ctx, &pb.Register{Username: "joao", Email: "joao@gmail.com", Password: "pass_joao"})

	slog.Debug("Register process sent for " + res.Username)
	if err != nil {
		log.Println("Register error:", err)
		return
	}
}
