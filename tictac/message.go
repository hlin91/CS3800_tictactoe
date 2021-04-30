// Package tictac provides functionality for implementing 2 player game of tic tac toe over TCP
// message.go defines the structure of the messages passed between client and server
package tictac

// ServerMessage contains the information the server sends to each client
type ServerMessage struct {
	Board    Board  // The board of the game
	PlayerID int    // The intended recipient of the message
	Ok       bool   // If false, the client should display the message and quit
	Message  string // Message to display
}

// ClientResponse contains the information the client sends to the server
type ClientResponse struct {
	Row      int
	Col      int
	PlayerID int  // The sender of the message
	Ok       bool // If false, the player has disconnected
}

//!--
