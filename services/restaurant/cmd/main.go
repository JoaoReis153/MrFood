package main

import (
	"MrFood/services/restaurant/internal/app"
)

func main() {

	go app.RunServer()

	app.RunClient()

}
