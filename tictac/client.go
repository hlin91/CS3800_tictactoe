// Package tictac provides functionality for implementing 2 player game of tic tac toe over TCP
// client.go implements a game client that connects to the master server
package tictac

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const (
	TIMEOUT = 10 // Number of seconds till timeout
)

type Client struct {
	board    Board    // The client's local copy of the board
	conn     net.Conn // The client's connection to the server
	clientID int      // The client's ID assigned by the server
}

// NewClient creates and returns a new client struct
func NewClient() Client {
	c := Client{}
	c.board = NewBoard()
	return c
}

// Connect attempts to connect the client to the server
func (c *Client) Connect() error {
	// Establish a tcp connection with the server
	conn, err := net.Dial("tcp", SERVER_IP+":"+SERVER_PORT)
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	var msg ServerMessage
	server := bufio.NewScanner(conn)
	server.Scan() // Get handshake message
	err = json.Unmarshal(server.Bytes(), &msg)
	if err != nil {
		return fmt.Errorf("failed to parse handshake: %v", err)
	}
	if !msg.Ok {
		return fmt.Errorf("failed to connect: %v", msg.Message)
	}
	// Successful connection
	// Initialize struct data
	c.clientID = msg.PlayerID
	c.conn = conn
	c.board.Clear()
	c.board.Display()
	fmt.Println(msg.Message)
	return nil
}

// Start starts the game of tic tac toe
func (c *Client) Start() error {
	defer c.conn.Close()
	var msg ServerMessage
	input := bufio.NewScanner(os.Stdin)
	server := bufio.NewScanner(c.conn)
	// Continue to listen for messages from the server
	for server.Scan() {
		// Unmarshal the json message from the server
		err := json.Unmarshal(server.Bytes(), &msg)
		if err != nil {
			return fmt.Errorf("failed to parse server message: %v", err)
		}
		// Load the board and print the text message
		c.board = msg.Board
		c.board.Clear()
		fmt.Println(msg.Message)
		c.board.Display()
		// Return if message is not ok
		if !msg.Ok {
			return fmt.Errorf(msg.Message)
		}
		mark := msg.Message[len(msg.Message)-1] // The mark the client uses to represent the player will be the last character in the message
		var row, col int
		valid := false
		// Get a valid row and column from user input
		for !valid {
			fmt.Print("Enter a row and column (eg. 0 1): ")
			input.Scan()
			tok := strings.Fields(strings.TrimSpace(input.Text()))
			if len(tok) != 2 {
				fmt.Println("Unexpected number of arguments. Try again.")
				continue
			}
			row, err = strconv.Atoi(tok[0])
			if err != nil {
				fmt.Println("Bad row. Try again.")
				continue
			}
			col, err = strconv.Atoi(tok[1])
			if err != nil {
				fmt.Println("Bad column. Try again.")
				continue
			}
			if !c.board.ValidTile(row, col) {
				fmt.Println("Bad tile. Try again.")
				continue
			}
			valid = true
		}
		// Construct a reply to send to the server containing the client selected row and column
		reply, err := json.Marshal(ClientResponse{Row: row, Col: col, PlayerID: c.clientID, Ok: true})
		if err != nil {
			return fmt.Errorf("failed to marshal client response: %v", err)
		}
		fmt.Fprintln(c.conn, string(reply)) // Send the reply to the server
		// Update the board locally and wait for a server reply
		c.board[row][col] = mark
		c.board.Clear()
		c.board.Display()
		fmt.Println("Waiting for server...")
	}
	return nil
}

//!--
