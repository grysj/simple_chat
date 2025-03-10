package main

import (
	"grysj/chat/internal/server"
)

func main() {
	server := server.NewServer("localhost:8080")
	if server != nil {
		server.Start()
	}
}
