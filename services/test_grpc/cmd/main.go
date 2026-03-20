package main

import (
	"MrFood/services/test_grpc/internal/app"
)

func main() {

	go app.RunServer()

	app.RunClient()

}
