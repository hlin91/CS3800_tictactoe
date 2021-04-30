// Package tictac provides functionality for implementing 2 player game of tic tac toe over TCP
// utility.go contains private utility functions for the package
package tictac

// Generates a message to the server indicating that the player has left
func leaveMessage(playerID int) ClientResponse {
	return ClientResponse{PlayerID: playerID, Ok: false}
}

// Generates a welcome message as a handshake for successfully connected players
func welcomeMessage(playerID int) ServerMessage {
	return ServerMessage{Board: NewBoard(), PlayerID: playerID, Ok: true, Message: "Welcome. Please wait for the game to start..."}
}
