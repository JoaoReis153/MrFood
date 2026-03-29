package app

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "MrFood/services/sponsor/internal/api/grpc/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func RunClient() {
	conn, err := grpc.NewClient("localhost:50055", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Fatal("failed to close grpc connection:", err)
		}
	}()

	client := pb.NewSponsorServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Example 1: 1 Ping and 1 Pong
	SinglePing(client, ctx)

	// // Example 2: 5 Pings and 1 Pong
	// MultiplePings(client, ctx)

	// // Example 3: 1 Ping and 5 Pongs
	// MultiplePongs(client, ctx)

	// // Example 4: 5 Ping and 5 Pongs
	// MultiplePingPongs(client, ctx)
}

func SinglePing(client pb.SponsorServiceClient, ctx context.Context) {
	fmt.Println("\n### Single ping single pong example ###")
	res, err := client.PingPong(ctx, &pb.Ping{Id: 1})
	fmt.Println("Ping 1")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Pong", res.Id)
}
