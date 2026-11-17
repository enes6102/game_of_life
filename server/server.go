package main

import (
	"flag"
	"net"
	"net/rpc"
	"os"
	"sync"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type GolOperations struct {
	mu     sync.Mutex
	cond   *sync.Cond
	world  [][]byte
	turn   int
	paused bool
	quit   bool
}

func (g *GolOperations) Evolve(req stubs.Request, res *stubs.Response) (err error) {
	g.mu.Lock()
	g.world, g.turn, g.paused, g.quit = req.World, 0, false, false
	g.mu.Unlock()

	for {
		g.mu.Lock()
		for g.paused && !g.quit {
			g.cond.Wait() // unlocks mu while waiting
		}
		if g.quit || g.turn >= req.Turns {
			res.World, res.CompletedTurns = g.world, g.turn
			g.mu.Unlock()
			return nil
		}
		world := g.world
		g.mu.Unlock()
		next := calculateNextState(world, 0, req.Height, req.Width, req.Height).strip

		g.mu.Lock()
		g.world = next
		g.turn++
		g.mu.Unlock()
	}
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

func (g *GolOperations) GetWorld(req stubs.Empty, res *stubs.WorldResponse) (err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	res.World, res.Turn = g.world, g.turn
	return nil
}

func (g *GolOperations) Pause(req stubs.Empty, res *stubs.PauseResponse) (err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.paused = !g.paused
	if !g.paused {
		g.cond.Broadcast()
	}
	res.Paused, res.Turn = g.paused, g.turn
	return nil
}

func (g *GolOperations) Quit(req stubs.Empty, res *stubs.Empty) (err error) {
	g.mu.Lock()
	g.quit = true
	g.cond.Broadcast() // wake evolve if paused
	g.mu.Unlock()
	return nil
}

func (g *GolOperations) Kill(req stubs.Empty, res *stubs.Empty) (err error) {
	go func() {
		time.Sleep(100 * time.Millisecond)
		os.Exit(0)
	}()
	return nil
}

func main() {
	port := flag.String("port", "8030", "port to listen on")
	flag.Parse()
	g := &GolOperations{}
	g.cond = sync.NewCond(&g.mu)
	rpc.Register(g)

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
