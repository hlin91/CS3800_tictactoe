// Package tictac provides functionality for implementing 2 player game of tic tac toe over TCP
// server.go implements the game master server
package tictac

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
)

const (
	REQUIRED_PLAYERS = 2
	SERVER_IP        = "127.0.0.1"
	SERVER_PORT      = "1337"
)

type Server struct {
	board       Board
	playerCount int
	players     []chan ServerMessage // Channels for sending messages to each connected player
	inMessages  chan ClientResponse  // Channel the server listens to for client responses
	sema        chan interface{}     // Semaphore for ensuring that goroutines return before killing main process
}

// NewServer creates and returns a new server
func NewServer() Server {
	s := Server{}
	s.board = NewBoard()
	s.inMessages = make(chan ClientResponse)
	s.sema = make(chan interface{}, REQUIRED_PLAYERS)
	s.players = []chan ServerMessage{}
	return s
}

// Run listens for player connections and runs the server once enough players have joined
func (s *Server) Run() error {
	log.Print("server running")
	listener, err := net.Listen("tcp", SERVER_IP+":"+SERVER_PORT)
	if err != nil {
		return fmt.Errorf("failed to run server: %v", err)
	}
	// Listen for players until we have enough
	log.Print("listening for player connections...")
	for s.playerCount < REQUIRED_PLAYERS {
		conn, err := listener.Accept()
		if err != nil {
			log.Print(err)
		}
		err = s.addPlayer(conn)
		if err != nil {
			log.Print(err)
		} else {
			log.Print("player successfully connected")
		}
	}
	log.Print("starting game")
	err = s.start()
	return err
}

// addPlayer adds a player to the server
func (s *Server) addPlayer(conn net.Conn) error {
	if s.playerCount == REQUIRED_PLAYERS {
		return fmt.Errorf("game is full")
	}
	// Create a channel for the player
	s.players = append(s.players, make(chan ServerMessage))
	// Start the handler goroutine to handle the connection
	go func(id int) {
		ch := s.players[id]
		handlePlayerConn(id, conn, ch, s.inMessages)
		var done interface{}
		s.sema <- done // Signal a return for the main goroutine
	}(s.playerCount)
	s.playerCount++
	return nil
}

// start starts the game of tic tac toe
func (s *Server) start() error {
	if s.playerCount != REQUIRED_PLAYERS {
		return fmt.Errorf("not enough players")
	}
	turn := 0
	marks := [2]byte{PLAYER_1_MARK, PLAYER_2_MARK}
	board := NewBoard()
	for {
		// Message each player in turn and request them to make a move
		msg := ServerMessage{
			Board:    board,
			PlayerID: turn,
			Ok:       true,
			Message:  fmt.Sprintf("Make your move %c", marks[turn]),
		}
		s.players[turn] <- msg
		// Wait for and get the reply
		reply := <-s.inMessages
		if !reply.Ok {
			// Tell clients that a player disconnected
			for _, ch := range s.players {
				ch <- ServerMessage{Board: s.board, PlayerID: 0, Ok: false, Message: fmt.Sprintf("player disconnected")}
			}
			return fmt.Errorf("a player disconnected")
		}
		if reply.PlayerID != turn {
			for _, ch := range s.players {
				ch <- ServerMessage{Board: s.board, PlayerID: 0, Ok: false, Message: fmt.Sprintf("player turns desynced")}
			}
			break
		}
		valid := board.ValidTile(reply.Row, reply.Col)
		// Resend request if client sends invalid tile position
		for !valid {
			msg.Message = fmt.Sprintf("Invalid tile, try again %c", marks[turn])
			s.players[turn] <- msg
			reply = <-s.inMessages
			if !reply.Ok {
				for _, ch := range s.players {
					ch <- ServerMessage{Board: s.board, PlayerID: 0, Ok: false, Message: fmt.Sprintf("player disconnected")}
				}
				for i := 0; i < REQUIRED_PLAYERS; i++ {
					<-s.sema
				}
				return fmt.Errorf("a player disconnected")
			}
			if reply.PlayerID != turn {
				for _, ch := range s.players {
					ch <- ServerMessage{Board: s.board, PlayerID: 0, Ok: false, Message: fmt.Sprintf("player disconnected")}
				}
				for i := 0; i < REQUIRED_PLAYERS; i++ {
					<-s.sema
				}
				return fmt.Errorf("turns are out of sync")
			}
			valid = board.ValidTile(reply.Row, reply.Col)
		}
		board[reply.Row][reply.Col] = marks[turn]
		victory := board.CheckVictory()
		if victory != 0 {
			// Someone won
			msg = ServerMessage{
				Board:   board,
				Ok:      false,
				Message: fmt.Sprintf("Player %d won!", victory),
			}
			for _, ch := range s.players {
				ch <- msg
			}
			log.Print(fmt.Sprintf("player %d won", victory))
			break
		}
		if board.CheckDraw() {
			// Its a draw
			msg = ServerMessage{
				Board:   board,
				Ok:      false,
				Message: "It's a draw!",
			}
			for _, ch := range s.players {
				ch <- msg
			}
			log.Print("it's a draw")
			break
		}
		turn = (turn + 1) % REQUIRED_PLAYERS
	}
	// Wait for all goroutines to finish
	for i := 0; i < REQUIRED_PLAYERS; i++ {
		<-s.sema
	}
	return nil
}

// Handle a player connection to the server
func handlePlayerConn(playerID int, conn net.Conn, inChan <-chan ServerMessage, outChan chan<- ClientResponse) {
	// Listen for messages from the server and forward them to the client
	// Then, listen for the client response and forward to the server
	// in JSON format
	// Write welcome message to the player
	defer conn.Close()
	welcome, err := json.Marshal(welcomeMessage(playerID))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprintln(conn, string(welcome))
	client := bufio.NewScanner(conn)
	for msg := range inChan {
		// Marshal the server message to JSON and send it to the player connection
		m, err := json.Marshal(msg)
		if err != nil {
			log.Fatal(fmt.Sprintf("handlePlayerConn: error marshaling message: %v", err))
		}
		_, err = fmt.Fprintln(conn, string(m)) // Write server message to client
		if err != nil {
			// Handle failure to write to connection as player leaving
			log.Println(fmt.Sprintf("handlePlayerConn: error writing to connection: %v", err))
			outChan <- leaveMessage(playerID) // Tell the server the player left
			return
		}
		client.Scan() // Get the client response
		var reply ClientResponse
		err = json.Unmarshal(client.Bytes(), &reply)
		if err != nil {
			// Failure to unmarshal likely means the player connection closed prematurely
			log.Println(fmt.Sprintf("handlePlayerConn: error unmarshaling response: %v", err))
			// Treat this as the player leaving
			outChan <- leaveMessage(playerID) // Tell the server the player left
			return
		}
		outChan <- reply
	}
	outChan <- leaveMessage(playerID) // Tell the server the player left
}

// Close closes all channels on the server
func (s *Server) Close() {
	for _, ch := range s.players {
		close(ch)
	}
	close(s.inMessages)
}

//!--
