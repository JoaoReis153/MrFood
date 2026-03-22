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
