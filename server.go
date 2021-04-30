// server.go implements the game server
package main

import (
	"fmt"

	"github.com/hlin91/CS3800_tictactoe/tictac"
)

func main() {
	server := tictac.NewServer()
	err := server.Run()
	if err != nil {
		fmt.Printf("server: %v", err)
	}
	server.Close()
}

//!--
