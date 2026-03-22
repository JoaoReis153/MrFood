package app

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "MrFood/services/auth/internal/api/grpc/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func (app *App) RunClient() {
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Fatal("failed to close grpc connection:", err)
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
	fmt.Println("Ping 1")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Pong", res.Id)
}

func RegisterProcess(client pb.TemplateServiceClient, ctx context.Context) {
	res, err := client.RegisterProcess(ctx, &pb.Register{Username: "joao", Email: "joao@gmail.com", Password: "pass_joao"})
	fmt.Println("Register sent for " + res.Username)
	if err != nil {
		log.Println("Register error:", err)
		return
	}
	fmt.Println("Register res", res)
}
