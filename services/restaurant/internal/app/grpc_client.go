package app

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "MrFood/services/restaurant/internal/api/grpc/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func RunClient() {
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Fatal("failed to close grpc connection:", err)
		}
	}()

	client := pb.NewRestaurantServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	GetRestaurant(client, ctx, 1)
	CompareRestaurants(client, ctx, 1, 2)
}

func GetRestaurant(client pb.RestaurantServiceClient, ctx context.Context, id int32) {
	fmt.Println("\n### Get restaurant details ###")
	res, err := client.GetRestaurantDetails(ctx, &pb.GetRestaurantDetailsRequest{Id: id})
	if err != nil {
		log.Println("GetRestaurantDetails failed:", err)
		return
	}

	restaurant := res.GetRestaurant()
	if restaurant == nil {
		fmt.Println("Restaurant not found")
		return
	}

	fmt.Printf("Restaurant #%d: %s\n", restaurant.GetId(), restaurant.GetName())
}

func CompareRestaurants(client pb.RestaurantServiceClient, ctx context.Context, id1, id2 int32) {
	fmt.Println("\n### Compare restaurants ###")
	res, err := client.CompareRestaurantDetails(ctx, &pb.CompareRestaurantDetailsRequest{
		RestaurantId_1: id1,
		RestaurantId_2: id2,
	})
	if err != nil {
		log.Println("CompareRestaurantDetails failed:", err)
		return
	}

	r1, r2 := res.GetRestaurant1(), res.GetRestaurant2()
	if r1 != nil {
		fmt.Printf("Restaurant 1 -> id=%d name=%s\n", r1.GetId(), r1.GetName())
	}
	if r2 != nil {
		fmt.Printf("Restaurant 2 -> id=%d name=%s\n", r2.GetId(), r2.GetName())
	}
}
