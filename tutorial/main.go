package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/danicat/simpleansi"
)

var maze []string

type sprite struct {
	row      int
	col      int
	startRow int
	startCol int
}

var player sprite
var ghosts []*ghost
var score int
var numDots int
var lives = 3

var (
	configFile = flag.String("config-file", "config.json", "path to custom config file")
	mazeFile   = flag.String("maze-file", "maze01.txt", "path to custom maze file")
)

type GhostStatus string

const (
	GhostStatusNormal GhostStatus = "Normal"
	GhostStatusBlue   GhostStatus = "Blue"
)

type ghost struct {
	position sprite
	status   GhostStatus
}

// Config holds the emoji configuration
type Config struct {
	Player              string        `json:"player"`
	Ghost               string        `json:"ghost"`
	Wall                string        `json:"wall"`
	Dot                 string        `json:"dot"`
	Pill                string        `json:"pill"`
	Death               string        `json:"death"`
	Space               string        `json:"space"`
	UseEmoji            bool          `json:"use_emoji"`
	GhostBlue           string        `json:"ghost_blue"`
	PillDurationSeconds time.Duration `json:"pill_duration_seconds"`
}

var cfg Config

var pillTimer *time.Timer
var pillMx sync.Mutex

func processPill() {
	pillMx.Lock()
	updateGhosts(ghosts, GhostStatusBlue)
	if pillTimer != nil {
		pillTimer.Stop()
	}

	pillTimer = time.NewTimer(time.Second * cfg.PillDurationSeconds)
	pillMx.Unlock()
	<-pillTimer.C
	pillMx.Lock()
	pillTimer.Stop()
	updateGhosts(ghosts, GhostStatusNormal)
	pillMx.Unlock()
}

var ghostStatusMx sync.RWMutex

func updateGhosts(ghosts []*ghost, ghostStatus GhostStatus) {

	ghostStatusMx.Lock()
	defer ghostStatusMx.Unlock()
	for _, g := range ghosts {
		g.status = ghostStatus
	}
}

func loadConfig(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	err = decoder.Decode(&cfg)
	if err != nil {
		return err
	}
	return nil
}

func loadMaze(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		maze = append(maze, line)
	}

	for row, line := range maze {
		for col, char := range line {
			switch char {
			case 'P':
				player = sprite{row: row, col: col, startRow: row, startCol: col}
			case 'G':
				ghosts = append(ghosts, &ghost{sprite{row, col, row, col}, GhostStatusNormal})
			case '.':
				numDots++
			}
		}
	}

	return nil
}

func printScreen() {
	simpleansi.ClearScreen()
	for _, line := range maze {
		for _, chr := range line {
			switch chr {
			case '#':
				fmt.Print(simpleansi.WithBlueBackground(cfg.Wall))
			case '.':
				fmt.Printf(cfg.Dot)
			default:
				fmt.Print(cfg.Space)
			}
		}
		fmt.Println()
	}

	moveCursor(player.row, player.col)
	fmt.Print(cfg.Player)

	for _, g := range ghosts {
		moveCursor(g.position.row, g.position.col)
		if g.status == GhostStatusNormal {
			fmt.Print(cfg.Ghost)
		} else if g.status == GhostStatusBlue {
			fmt.Print(cfg.GhostBlue)
		}
	}
	moveCursor(len(maze)+1, 0)

	livesRemaining := strconv.Itoa(lives)
	if cfg.UseEmoji {
		livesRemaining = getLivesAsEmoji()
	}
	fmt.Println("Score:", score, "\tLives", livesRemaining)

}

func getLivesAsEmoji() string {
	buffer := bytes.Buffer{}
	for i := lives; i > 0; i-- {
		buffer.WriteString(cfg.Player)
	}
	return buffer.String()
}

func drawDirection() string {
	dir := rand.Intn(4)
	move := map[int]string{
		0: "UP",
		1: "DOWN",
		2: "RIGHT",
		3: "LEFT",
	}
	return move[dir]
}

func moveGhosts() {
	for _, g := range ghosts {
		dir := drawDirection()
		g.position.row, g.position.col = makeMove(g.position.row, g.position.col, dir)
	}
}

func initialize() {
	cbTerm := exec.Command("stty", "cbreak", "-echo")
	cbTerm.Stdin = os.Stdin

	err := cbTerm.Run()
	if err != nil {
		log.Fatalln("unable to activate cbreak mode:", err)
	}
}

func cleanup() {
	cookedTerm := exec.Command("stty", "-cbreak", "echo")
	cookedTerm.Stdin = os.Stdin

	err := cookedTerm.Run()
	if err != nil {
		log.Fatalln("unable to restore cooked mode:", err)
	}
}

func movePlayer(dir string) {
	player.row, player.col = makeMove(player.row, player.col, dir)
	removeDot := func(row, col int) {
		maze[row] = maze[row][0:col] + " " + maze[row][col+1:]
	}
	switch maze[player.row][player.col] {
	case '.':
		numDots--
		score++
		maze[player.row] = maze[player.row][0:player.col] + " " + maze[player.row][player.col+1:]
	case 'X':
		score += 10
		removeDot(player.row, player.col)
		go processPill()
	}
}

func makeMove(oldRow, oldCol int, dir string) (newRow, newCol int) {
	newRow, newCol = oldRow, oldCol

	switch dir {
	case "UP":
		newRow = newRow - 1
		if newRow < 0 {
			newRow = len(maze) - 1
		}
	case "DOWN":
		newRow = newRow + 1
		if newRow == len(maze) {
			newRow = 0
		}
	case "RIGHT":
		newCol = newCol + 1
		if newCol == len(maze[0]) {
			newCol = 0
		}
	case "LEFT":
		newCol = newCol - 1
		if newCol < 0 {
			newCol = len(maze[0]) - 1
		}
	}

	if maze[newRow][newCol] == '#' {
		newRow = oldRow
		newCol = oldCol
	}

	return
}
func readInput() (string, error) {
	buffer := make([]byte, 100)

	cnt, err := os.Stdin.Read(buffer)
	if err != nil {
		return "", nil
	}

	if cnt == 1 && buffer[0] == 0x1b {
		return "ESC", nil
	} else if cnt >= 3 {
		if buffer[0] == 0x1b && buffer[1] == '[' {
			switch buffer[2] {
			case 'A':
				return "UP", nil
			case 'B':
				return "DOWN", nil
			case 'C':
				return "RIGHT", nil
			case 'D':
				return "LEFT", nil
			}
		}
	}

	return "", nil
}

func moveCursor(row, col int) {
	if cfg.UseEmoji {
		simpleansi.MoveCursor(row, col*2)
	} else {
		simpleansi.MoveCursor(row, col)
	}
}

func main() {
	flag.Parse()
	initialize()
	defer cleanup()

	err := loadMaze(*mazeFile)
	if err != nil {
		log.Println("failed to load maze:", err)
		return
	}

	err = loadConfig(*configFile)
	if err != nil {
		log.Println("failed to load config:", err)
		return
	}

	input := make(chan string)
	go func(ch chan<- string) {
		for {
			input, err := readInput()
			if err != nil {
				log.Println("error reading input:", err)
				ch <- "ESC"
			}
			ch <- input
		}
	}(input)

	for {

		select {
		case inp := <-input:
			if inp == "ESC" {
				lives = 0
			}
			movePlayer(inp)
		default:
		}

		moveGhosts()

		for _, g := range ghosts {
			if player.row == g.position.row && player.col == g.position.col {
				lives = lives - 1
				if lives != 0 {
					moveCursor(player.row, player.col)
					fmt.Print(cfg.Death)
					moveCursor(len(maze)+2, 0)
					time.Sleep(1000 * time.Microsecond)
					player.row, player.col = player.startRow, player.startRow
				}
			}
		}

		printScreen()

		if numDots == 0 || lives == 0 {
			if lives == 0 {
				moveCursor(player.row, player.col)
				fmt.Print(cfg.Death)
				moveCursor(len(maze)+2, 0)
			}
			break
		}

		time.Sleep(200 * time.Millisecond)
	}
}
