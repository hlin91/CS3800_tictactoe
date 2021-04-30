// client.go implements the game server
package main

import (
	"fmt"

	"github.com/hlin91/CS3800_tictactoe/tictac"
)

func main() {
	client := tictac.NewClient()
	err := client.Connect()
	if err != nil {
		fmt.Printf("client: %v", err)
		return
	}
	err = client.Start()
	if err != nil {
		fmt.Printf("client: %v", err)
	}
}

//!--
