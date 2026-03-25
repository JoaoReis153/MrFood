package app

import (
	"context"
	"fmt"
	"io"
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

func MultiplePings(client pb.SponsorServiceClient, ctx context.Context) {
	fmt.Println("\n### Multiple pings single pong example ###")
	stream, err := client.ManyPings(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Send pings
	for i := 0; i < 5; i++ {
		if err := stream.Send(&pb.Ping{Id: int32(i)}); err != nil {
			log.Fatal(err)
		}
	}

	// Close the stream and receive response
	res, err := stream.CloseAndRecv()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("ManyPings response:", res.Id)
}

func MultiplePongs(client pb.SponsorServiceClient, ctx context.Context) {
	fmt.Println("\n### Single ping multiple pongs example ###")
	stream, err := client.ManyPongs(ctx, &pb.Ping{Id: 1})
	if err != nil {
		log.Fatal(err)
	}

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("ManyPongs received:", msg.Id)
	}
}

func MultiplePingPongs(client pb.SponsorServiceClient, ctx context.Context) {
	fmt.Println("\n### Multiple ping multiple pongs example ###")
	stream, err := client.ManyPingPongs(ctx)
	if err != nil {
		log.Fatal(err)
	}

	waitc := make(chan struct{})

	// Send pings
	go func() {
		for i := 0; i < 5; i++ {
			if err := stream.Send(&pb.Ping{Id: int32(i)}); err != nil {
				log.Fatal(err)
			}
		}
		// Close the send stream
		err = stream.CloseSend()
	}()

	// Receive pongs
	go func() {
		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println("Pongs received:", msg.Id)
		}
		close(waitc)
	}()

	// Wait for the receiving goroutine to finish
	<-waitc
}
