package gol

import (
	"fmt"
	"net/rpc"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
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
	keyPresses <-chan rune // step 5
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

	client, err := rpc.Dial("tcp", *brokerAddr)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	req := stubs.Request{World: world, Width: p.ImageWidth, Height: p.ImageHeight, Turns: p.Turns}
	res := new(stubs.Response)
	evolveCall := client.Go(stubs.EvolveHandler, req, res, nil) // non-blocking

	running := true
	kill := false
	for running {
		select {
		case <-evolveCall.Done:
			if evolveCall.Error != nil {
				panic(evolveCall.Error)
			}
			running = false
		case <-ticker.C:
			aliveRes := new(stubs.AliveResponse)
			err := client.Call(stubs.AliveHandler, stubs.AliveRequest{}, aliveRes)
			if err != nil {
				panic(err)
			}
			c.events <- AliveCellsCount{
				CompletedTurns: aliveRes.CompletedTurns,
				CellsCount:     aliveRes.CellsCount,
			}
		case key := <-c.keyPresses:
			switch key {
			case 's':
				w := new(stubs.WorldResponse)
				err := client.Call(stubs.GetWorldHandler, stubs.Empty{}, w)
				if err != nil {
					panic(err)
				}
				saveImage(p, c, w.World, w.Turn)
			case 'q':
				err := client.Call(stubs.QuitHandler, stubs.Empty{}, new(stubs.Empty))
				if err != nil {
					return
				}
				// evolv now returns, the done case ends the loop
			case 'k':
				kill = true
				err := client.Call(stubs.QuitHandler, stubs.Empty{}, new(stubs.Empty))
				if err != nil {
					return
				}
			case 'p':
				pr := new(stubs.PauseResponse)
				err := client.Call(stubs.PauseHandler, stubs.Empty{}, pr)
				if err != nil {
					return
				}
				if pr.Paused {
					fmt.Println("Paused at turn", pr.Turn)
					c.events <- StateChange{pr.Turn, Paused}
				} else {
					fmt.Println("Continuing")
					c.events <- StateChange{pr.Turn, Executing}
				}
			}
		}

	}

	world = res.World
	turn = res.CompletedTurns

	// TODO: Report the final state using FinalTurnCompleteEvent.
	c.events <- FinalTurnComplete{
		CompletedTurns: turn,
		Alive:          calculateAliveCells(world, p.ImageWidth, p.ImageHeight),
	}

	saveImage(p, c, world, turn)

	c.events <- StateChange{
		turn, Quitting}

	if kill {
		client.Call(stubs.KillHandler, stubs.Empty{}, new(stubs.Empty))
	}

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
