package main

import (
	"MrFood/services/sponsor/internal/app"
)

func main() {

	go app.RunServer()

	app.RunClient()

}
