package main

import (
	"fmt"
	client "grysj/chat/internal/client"
)

func main() {

	client, err := client.NewClient()
	if err != nil {
		fmt.Println("error: ", err)
		return
	}

	err = client.ConnectToServerTCP("localhost:8080")
	if err != nil {
		fmt.Println("error: ", err)
		return
	}

	go client.HandleIncomingTCP()
	go client.HandleIncomingUDP()
	go client.HandleIncomingMulticast()
	client.HandleOutgoing()

}
