package main

import (
	"MrFood/services/review/internal/app"
)

func main() {

	go app.RunServer()

	app.RunClient()

}
