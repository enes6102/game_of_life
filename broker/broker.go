package main

import (
	"flag"
	"net"
	"net/rpc"
	"os"
	"strings"
	"sync"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
)

// GolOperations is the broker. The local controller talks only to this;
// it keeps the game state and farms each turn out to the worker machines.
type GolOperations struct {
	mu      sync.Mutex
	cond    *sync.Cond
	world   [][]byte
	turn    int
	paused  bool
	quit    bool
	workers []*rpc.Client
}

func (g *GolOperations) Evolve(req stubs.Request, res *stubs.Response) (err error) {
	g.mu.Lock()
	g.world, g.turn, g.paused, g.quit = req.World, 0, false, false // new controller = fresh state
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

		next, err := g.step(world, req.Width, req.Height)
		if err != nil {
			return err
		}

		g.mu.Lock()
		g.world = next
		g.turn++
		g.mu.Unlock()
	}
}

// step computes one turn by splitting the board into strips, one per worker.
// Each worker gets its strip plus one halo row above and below (wrapping).
func (g *GolOperations) step(world [][]byte, width, height int) ([][]byte, error) {
	n := len(g.workers)
	if n > height {
		n = height // never give a worker 0 rows
	}
	calls := make([]*rpc.Call, n)
	resps := make([]*stubs.WorkerResponse, n)

	base := height / n
	remainder := height % n
	startY := 0

	for i := 0; i < n; i++ {
		rows := base
		if i < remainder {
			rows++ // first 'remainder' workers take one extra row
		}
		endY := startY + rows

		strip := make([][]byte, 0, rows+2)
		strip = append(strip, world[(startY-1+height)%height]) // halo above
		strip = append(strip, world[startY:endY]...)
		strip = append(strip, world[endY%height]) // halo below

		resps[i] = new(stubs.WorkerResponse)
		calls[i] = g.workers[i].Go(stubs.WorkerUpdateHandler,
			stubs.WorkerRequest{Rows: strip, Width: width}, resps[i], nil) // non-blocking: all workers run at once

		startY = endY
	}

	// collect strips in order and glue them together
	next := make([][]byte, 0, height)
	for i := 0; i < n; i++ {
		<-calls[i].Done
		if calls[i].Error != nil {
			return nil, calls[i].Error
		}
		next = append(next, resps[i].Strip...)
	}
	return next, nil
}

func (g *GolOperations) AliveCount(req stubs.AliveRequest, res *stubs.AliveResponse) (err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	res.CompletedTurns = g.turn
	res.CellsCount = countAlive(g.world)
	return nil
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
	g.cond.Broadcast() // wake Evolve if paused
	g.mu.Unlock()
	return nil
}

// Kill shuts down every worker, then the broker itself.
func (g *GolOperations) Kill(req stubs.Empty, res *stubs.Empty) (err error) {
	for _, w := range g.workers {
		w.Call(stubs.WorkerKillHandler, stubs.Empty{}, new(stubs.Empty))
	}
	go func() {
		time.Sleep(100 * time.Millisecond) // let this reply reach the controller first
		os.Exit(0)
	}()
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
	workerAddrs := flag.String("workers", "127.0.0.1:8031", "comma-separated worker addresses (ip:port)")
	flag.Parse()

	g := &GolOperations{}
	g.cond = sync.NewCond(&g.mu)
	for _, addr := range strings.Split(*workerAddrs, ",") {
		c, err := rpc.Dial("tcp", strings.TrimSpace(addr))
		if err != nil {
			panic(err)
		}
		g.workers = append(g.workers, c)
	}

	rpc.Register(g)
	ln, err := net.Listen("tcp", ":"+*port)
	if err != nil {
		panic(err)
	}
	defer ln.Close()
	rpc.Accept(ln)
}
