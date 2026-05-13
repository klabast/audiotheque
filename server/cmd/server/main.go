package main

import (
	"audiod/cmd/server/commands"
	"log"
)

// @title Audiotheque API
// @version 2.0
// @description Audiotheque music server API
// @host localhost:8080
// @BasePath /api
func main() {
	if err := commands.Execute(); err != nil {
		log.Fatal(err)
	}
}
