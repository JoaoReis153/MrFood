package main

import (
	"MrFood/services/booking/internal/app"
)

func main() {

	go app.RunServer()

	app.RunClient()

}
