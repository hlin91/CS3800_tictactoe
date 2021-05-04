// Package tictac provides functionality for implementing 2 player game of tic tac toe over TCP
// board.go implements the game board
package tictac

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

const (
	BOARD_SIZE    = 3
	PLAYER_1_MARK = 'X'
	PLAYER_2_MARK = 'O'
	EMPTY_TILE    = '_'
)

type Board [BOARD_SIZE][BOARD_SIZE]byte

// NewBoard creates and returns a new board
func NewBoard() Board {
	var b Board
	for i := 0; i < BOARD_SIZE; i++ {
		for j := 0; j < BOARD_SIZE; j++ {
			b[i][j] = EMPTY_TILE
		}
	}
	return b
}

// Display draws the contents of the board to the screen
func (b Board) Display() {
	for _, row := range b {
		for _, tile := range row {
			fmt.Printf(" %c", tile)
		}
		fmt.Println()
	}
}

// ValidTile checks if the position denotes a valid tile on the board
func (b Board) ValidTile(row, col int) bool {
	return row >= 0 && row < BOARD_SIZE && col >= 0 && col < BOARD_SIZE && b[row][col] == EMPTY_TILE
}

// CheckVictory checks if a player won. Returns 1 if player 1 won and 2 if player 2 won.
// If there is no victory, it returns 0
func (b Board) CheckVictory() int {
	// Check the rows
	sum := 0
	for _, row := range b {
		sum = 0
		for _, col := range row {
			sum += int(col)
		}
		if sum%PLAYER_1_MARK == 0 {
			return 1
		}
		if sum%PLAYER_2_MARK == 0 {
			return 2
		}
	}
	// Check the columns
	for i := 0; i < BOARD_SIZE; i++ {
		sum = 0
		for j := 0; j < BOARD_SIZE; j++ {
			sum += int(b[j][i])
		}
		if sum%PLAYER_1_MARK == 0 {
			return 1
		}
		if sum%PLAYER_2_MARK == 0 {
			return 2
		}
	}
	// Check the diagonals
	sum = 0
	for i := 0; i < BOARD_SIZE; i++ {
		sum += int(b[i][i])
	}
	if sum%PLAYER_1_MARK == 0 {
		return 1
	}
	if sum%PLAYER_2_MARK == 0 {
		return 2
	}
	sum = 0
	for i := BOARD_SIZE - 1; i >= 0; i-- {
		sum += int(b[i][BOARD_SIZE-i-1])
	}
	if sum%PLAYER_1_MARK == 0 {
		return 1
	}
	if sum%PLAYER_2_MARK == 0 {
		return 2
	}
	return 0
}

// CheckDraw checks if there is a draw
func (b Board) CheckDraw() bool {
	for _, row := range b {
		for _, col := range row {
			if col == EMPTY_TILE {
				return false
			}
		}
	}
	return true
}

// Clear clears the screen
func (b Board) Clear() {
	switch runtime.GOOS {
	case "linux":
		fallthrough
	case "darwin":
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		cmd.Run()
	case "windows":
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	}
}

//!--
