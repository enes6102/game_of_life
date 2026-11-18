package gol

import (
	"fmt"
	"time"

	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	keyPresses <-chan rune // step 5
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	// TODO: Create a 2D slice to store the world.

	world := make([][]byte, p.ImageHeight)
	for i := range world {
		world[i] = make([]byte, p.ImageWidth)
	}

	c.ioCommand <- ioInput
	c.ioFilename <- fmt.Sprintf("%dx%d", p.ImageWidth, p.ImageHeight)

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			world[y][x] = <-c.ioInput
		}
	}

	turn := 0
	// step 6: tell SDL which cells start alive
	initialAlive := calculateAliveCells(world, p.ImageWidth, p.ImageHeight)
	if len(initialAlive) > 0 {
		c.events <- CellsFlipped{CompletedTurns: turn, Cells: initialAlive}
	}
	c.events <- StateChange{turn, Executing}

	// TODO: Execute all turns of the Game of Life.
	//step 3
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop() // stops ticker when distributor returns so it doesnt keep running in the background

	quit := false
	for turn < p.Turns && !quit {
		channels := make([]chan workerResult, p.Threads)

		base := p.ImageHeight / p.Threads
		remainder := p.ImageHeight % p.Threads
		startY := 0

		for i := 0; i < p.Threads; i++ {
			rows := base
			if i < remainder {
				rows++ // first 'remainder' workers take one extra row
			}
			endY := startY + rows

			channels[i] = make(chan workerResult)
			go worker(world, startY, endY, p.ImageWidth, p.ImageHeight, channels[i])

			startY = endY
		}

		// collect strips .in order. and glue them together
		newWorld := make([][]byte, 0, p.ImageHeight)
		var flipped []util.Cell
		for i := 0; i < p.Threads; i++ {
			res := <-channels[i]
			newWorld = append(newWorld, res.strip...)
			flipped = append(flipped, res.flipped...)
		}

		world = newWorld
		turn++

		if len(flipped) > 0 {
			c.events <- CellsFlipped{CompletedTurns: turn, Cells: flipped}
		}
		c.events <- TurnComplete{CompletedTurns: turn}

		//step 3 + 5:
		select {
		case <-ticker.C:
			c.events <- AliveCellsCount{
				CompletedTurns: turn,
				CellsCount:     countAlive(world),
			}
		case key := <-c.keyPresses:
			switch key {
			case 's':
				saveImage(p, c, world, turn)
			case 'q':
				quit = true
			case 'p':
				quit = pause(p, c, world, turn)
			}
		default:
			//carry on
		}

	}

	// TODO: Report the final state using FinalTurnCompleteEvent.
	c.events <- FinalTurnComplete{
		CompletedTurns: turn,
		Alive:          calculateAliveCells(world, p.ImageWidth, p.ImageHeight),
	}

	saveImage(p, c, world, turn)

	c.events <- StateChange{
		turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

// countAliveNeighbours counts the 8 surrounding cells, wrapping at the edges.
func countAliveNeighbours(world [][]byte, x, y, width, height int) int {
	count := 0
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue // skip the cell itself
			}
			nx := (x + dx + width) % width   // wrap horizontally
			ny := (y + dy + height) % height // wrap vertically
			if world[ny][nx] == 255 {
				count++
			}
		}
	}
	return count
}

type workerResult struct {
	strip   [][]byte
	flipped []util.Cell
}

func calculateNextState(world [][]byte, startY, endY, width, height int) workerResult {
	strip := make([][]byte, endY-startY)
	for i := range strip {
		strip[i] = make([]byte, width)
	}

	var flipped []util.Cell

	for y := startY; y < endY; y++ {
		for x := 0; x < width; x++ {
			n := countAliveNeighbours(world, x, y, width, height)
			alive := world[y][x] == 255
			var next byte
			if (alive && (n == 2 || n == 3)) || (!alive && n == 3) {
				next = 255
			}
			strip[y-startY][x] = next
			if next != world[y][x] {
				flipped = append(flipped, util.Cell{X: x, Y: y})
			}
		}
	}
	return workerResult{strip: strip, flipped: flipped}
}

// worker computes one strip and sends it back on its own channel.
func worker(world [][]byte, startY, endY, width, height int, out chan<- workerResult) {
	out <- calculateNextState(world, startY, endY, width, height)
}

// calculateAliveCells returns the coordinates of every alive cell.
func calculateAliveCells(world [][]byte, width, height int) []util.Cell {
	var alive []util.Cell
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if world[y][x] == 255 {
				alive = append(alive, util.Cell{X: x, Y: y})
			}
		}
	}
	return alive
}

func countAlive(world [][]byte) int {
	count := 0
	for _, row := range world {
		for _, cell := range row {
			if cell == 255 {
				count++
			}
		}
	}
	return count
}

func saveImage(p Params, c distributorChannels, world [][]byte, turn int) {
	filename := fmt.Sprintf("%dx%dx%d", p.ImageWidth, p.ImageHeight, turn)

	c.ioCommand <- ioOutput
	c.ioFilename <- filename
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}

	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- ImageOutputComplete{CompletedTurns: turn, Filename: filename}
}

func pause(p Params, c distributorChannels, world [][]byte, turn int) bool {
	c.events <- StateChange{turn, Paused}
	for {
		switch <-c.keyPresses {
		case 's':
			saveImage(p, c, world, turn)
		case 'q':
			return true
		case 'p':
			c.events <- StateChange{turn, Executing}
			return false
		}
	}
}
