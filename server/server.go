package main

import (
	"flag"
	"net"
	"net/rpc"
	"sync"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type GolOperations struct {
	mu    sync.Mutex
	world [][]byte
	turn  int
}

func (g *GolOperations) Evolve(req stubs.Request, res *stubs.Response) (err error) {
	g.mu.Lock()
	g.world = req.World
	g.turn = 0
	g.mu.Unlock()

	world := req.World
	turn := 0
	for ; turn < req.Turns; turn++ {
		// split into req.threads strips, start workers, glue strips into newworld
		world = calculateNextState(world, 0, req.Height, req.Width, req.Height).strip
	}
	res.World = world
	res.CompletedTurns = turn
	return nil

	//if len(flipped) > 0 {
	//	c.events <- CellsFlipped{CompletedTurns: turn, Cells: flipped}
	//}
	//c.events <- TurnComplete{CompletedTurns: turn}

	//step 3 + 5:
	//select {
	//case <-ticker.C:
	//	c.events <- AliveCellsCount{
	//		CompletedTurns: turn,
	//		CellsCount:     countAlive(world),
	//	}
	//case key := <-c.keyPresses:
	//	switch key {
	//	case 's':
	//		saveImage(p, c, world, turn)
	//	case 'q':
	//		quit = true
	//	case 'p':
	//		quit = pause(p, c, world, turn)
	//	}
	//default:
	//	//carry on
	//}
}

func (g *GolOperations) AliveCount(req stubs.AliveRequest, res *stubs.AliveResponse) (err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	res.CompletedTurns = g.turn
	res.CellsCount = countAlive(g.world)
	return nil
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

func main() {
	port := flag.String("port", "8030", "port to listen on")
	flag.Parse()
	rpc.Register(&GolOperations{})
	ln, _ := net.Listen("tcp", ":"+*port)
	defer ln.Close()
	rpc.Accept(ln)
}

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
