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
	// NOTE: These attributes all get passed to the game instance. The server does not interact with these directly outside of playerCount
	playerCount int                  // Number of players currently joined
	players     []chan ServerMessage // Channels for sending messages to each connected player
	inMessages  chan ClientResponse  // Channel to store messages received from the clients
	sema        chan interface{}     // Semaphore used to wait until all goroutines finish
}

// NewServer creates and returns a new server
func NewServer() Server {
	s := Server{}
	s.players = []chan ServerMessage{}
	s.inMessages = make(chan ClientResponse, 1)
	s.sema = make(chan interface{}, REQUIRED_PLAYERS)
	return s
}

// Run listens for player connections and starts a game instance once enough players have joined
func (s *Server) Run() error {
	listener, err := net.Listen("tcp", SERVER_IP+":"+SERVER_PORT)
	if err != nil {
		return fmt.Errorf("failed to run server: %v", err)
	}
	for true {
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
		// Start a game instance once enough players have joined
		log.Print("starting game instance")
		go func(players []chan ServerMessage, playerCount int, inMessages chan ClientResponse, sema chan interface{}) {
			game := newGameInstance(players, playerCount, inMessages, sema)
			err := game.start()
			if err != nil {
				log.Printf("gameInstance: %v\n", err)
			}
			game.Close()
		}(s.players, s.playerCount, s.inMessages, s.sema)
		// Continue listening for players
		s.reset()
	}
	return nil
}

// reset resets the players and channels of the server
func (s *Server) reset() {
	s.playerCount = 0
	s.players = []chan ServerMessage{}
	s.inMessages = make(chan ClientResponse)
	s.sema = make(chan interface{}, REQUIRED_PLAYERS)
}

// addPlayer adds a player to the server
func (s *Server) addPlayer(conn net.Conn) error {
	if s.playerCount == REQUIRED_PLAYERS {
		return fmt.Errorf("game is full")
	}
	// Create a channel for the player
	s.players = append(s.players, make(chan ServerMessage, 1))
	// Start the handler goroutine to handle the connection
	go func(id int, sema chan interface{}, inMessages chan ClientResponse, players []chan ServerMessage) {
		ch := players[id]
		handlePlayerConn(id, conn, ch, inMessages)
		var done interface{}
		sema <- done // Signal a return for the main goroutine
	}(s.playerCount, s.sema, s.inMessages, s.players)
	s.playerCount++
	return nil
}

// gameInstance represents a single game of tic tac toe between two players
type gameInstance struct {
	board       Board                // The tic-tac-toe board
	playerCount int                  // Number of players in the game
	players     []chan ServerMessage // Channels for each player to store server messages
	inMessages  chan ClientResponse  // Channel to store responses from the clients
	sema        chan interface{}     // Semaphore used to wait until all goroutines finish
}

// newGameInstance create a new game instance with the passed players
func newGameInstance(p []chan ServerMessage, count int, messages chan ClientResponse, semaphore chan interface{}) gameInstance {
	return gameInstance{
		board:       NewBoard(),
		playerCount: count,
		players:     p,
		inMessages:  messages,
		sema:        semaphore,
	}
}

// start starts the game of tic tac toe
func (g *gameInstance) start() error {
	if g.playerCount != REQUIRED_PLAYERS {
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
		g.players[turn] <- msg
		// Wait for and get the reply
		reply := <-g.inMessages
		if !reply.Ok {
			// Tell clients that a player disconnected
			g.broadcast(ServerMessage{Board: g.board, PlayerID: 0, Ok: false, Message: fmt.Sprintf("player disconnected")})
			// Wait for all goroutines to finish
			g.wait()
			return nil
		}
		if reply.PlayerID != turn {
			g.broadcast(ServerMessage{Board: g.board, PlayerID: 0, Ok: false, Message: fmt.Sprintf("player turns desynced")})
			// Wait for all goroutines to finish
			g.wait()
			return fmt.Errorf("player turns desynced")
		}
		valid := board.ValidTile(reply.Row, reply.Col)
		// Resend request if client sends invalid tile position
		for !valid {
			msg.Message = fmt.Sprintf("Invalid tile, try again %c", marks[turn])
			g.players[turn] <- msg
			reply = <-g.inMessages
			if !reply.Ok {
				g.broadcast(ServerMessage{Board: g.board, PlayerID: 0, Ok: false, Message: fmt.Sprintf("player disconnected")})
				g.wait()
				return fmt.Errorf("a player disconnected")
			}
			if reply.PlayerID != turn {
				g.broadcast(ServerMessage{Board: g.board, PlayerID: 0, Ok: false, Message: fmt.Sprintf("player disconnected")})
				g.wait()
				return fmt.Errorf("player turns desynced")
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
			g.broadcast(msg)
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
			g.broadcast(msg)
			log.Print("it's a draw")
			break
		}
		turn = (turn + 1) % REQUIRED_PLAYERS
	}
	g.wait()
	return nil
}

// broadcast sends a  message to all player channels
func (g gameInstance) broadcast(msg ServerMessage) {
	for _, ch := range g.players {
		ch <- msg
	}
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
			log.Println("handlePlayerConn: player connection lost")
			// Treat this as the player leaving
			outChan <- leaveMessage(playerID) // Tell the server the player left
			return
		}
		outChan <- reply
	}
	outChan <- leaveMessage(playerID) // Tell the server the player left
}

// wait waits for goroutines to return via the semaphore
func (g *gameInstance) wait() {
	for i := 0; i < REQUIRED_PLAYERS; i++ {
		<-g.sema
	}
}

// Close closes all channels on the game instance
func (g *gameInstance) Close() {
	for _, ch := range g.players {
		close(ch)
	}
	close(g.inMessages)
}

//!--
