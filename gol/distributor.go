package gol

import (
	"fmt"
	"time"

	"uk.ac.bris.cs/gameoflife/util"
)

// file to change
type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	// TODO: Create a 2D slice to store the world.

	world := make([][]byte, p.ImageHeight)
	for i := range world {
		world[i] = make([]byte, p.ImageWidth)
	}
	// read pgm and make the world
	c.ioCommand <- ioInput
	c.ioFilename <- fmt.Sprintf("%dx%d", p.ImageWidth, p.ImageHeight)

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			world[y][x] = <-c.ioInput
		}
	}

	// read pgm and make the world

	turn := 0
	c.events <- StateChange{turn, Executing}

	// TODO: Execute all turns of the Game of Life.
	//step 3
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop() // stops ticker when distributor returns so it doesnt keep running in the background

	for turn < p.Turns {
		//world = calculateNextState(world, p.ImageWidth, p.ImageHeight)
		//turn++
		channels := make([]chan [][]byte, p.Threads)

		base := p.ImageHeight / p.Threads
		remainder := p.ImageHeight % p.Threads
		startY := 0

		for i := 0; i < p.Threads; i++ {
			rows := base
			if i < remainder {
				rows++ // first 'remainder' workers take one extra row
			}
			endY := startY + rows

			channels[i] = make(chan [][]byte)
			go worker(world, startY, endY, p.ImageWidth, p.ImageHeight, channels[i])

			startY = endY
		}

		// collect strips .in order. and glue them together
		newWorld := make([][]byte, 0, p.ImageHeight)
		for i := 0; i < p.Threads; i++ {
			strip := <-channels[i]
			newWorld = append(newWorld, strip...)
		}

		world = newWorld
		turn++

		//step 3:
		select {
		case <-ticker.C:
			c.events <- AliveCellsCount{
				CompletedTurns: turn,
				CellsCount:     countAlive(world),
			}
		default:
			//no tick yet so carry on
		}

	}

	// TODO: Report the final state using FinalTurnCompleteEvent.
	c.events <- FinalTurnComplete{
		CompletedTurns: turn,
		Alive:          calculateAliveCells(world, p.ImageWidth, p.ImageHeight),
	}
	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

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

// calculateNextState builds a brand-new world from the old one.
func calculateNextState(world [][]byte, startY, endY, width, height int) [][]byte {
	strip := make([][]byte, endY-startY)
	for i := range strip {
		strip[i] = make([]byte, width)
	}

	for y := startY; y < endY; y++ {
		for x := 0; x < width; x++ {
			n := countAliveNeighbours(world, x, y, width, height)
			alive := world[y][x] == 255
			if (alive && (n == 2 || n == 3)) || (!alive && n == 3) {
				strip[y-startY][x] = 255
			} else {
				strip[y-startY][x] = 0
			}
		}
	}
	return strip
}

// worker computes one strip and sends it back on its own channel.
func worker(world [][]byte, startY, endY, width, height int, out chan<- [][]byte) {
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
