package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"runtime"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
)

// Worker receives a strip of the board (plus one halo row above and below),
// computes one turn for it, and sends the new strip back to the broker.
type Worker struct {
	threads int // goroutines used inside this worker, like p.Threads in the parallel version
}

// Update computes the next state of the strip in req.Rows.
// req.Rows[0] and req.Rows[len-1] are halo rows: they are read but not returned.
func (w *Worker) Update(req stubs.WorkerRequest, res *stubs.WorkerResponse) (err error) {
	host, _ := os.Hostname()
	fmt.Printf("[%s] got %d rows\n", host, len(req.Rows)-2)
	rows, width := req.Rows, req.Width
	n := len(rows) - 2 // number of real rows this worker owns

	threads := w.threads
	if threads > n {
		threads = n // never give a goroutine 0 rows
	}
	if threads < 1 {
		threads = 1
	}

	// same split as the parallel distributor, but over rows 1..n (skipping the halos)
	channels := make([]chan [][]byte, threads)
	base := n / threads
	remainder := n % threads
	startY := 1

	for i := 0; i < threads; i++ {
		count := base
		if i < remainder {
			count++ // first 'remainder' goroutines take one extra row
		}
		endY := startY + count

		channels[i] = make(chan [][]byte)
		go worker(rows, startY, endY, width, channels[i])

		startY = endY
	}

	// collect strips in order and glue them together
	strip := make([][]byte, 0, n)
	for i := 0; i < threads; i++ {
		strip = append(strip, <-channels[i]...)
	}

	res.Strip = strip
	return nil
}

func (w *Worker) Kill(req stubs.Empty, res *stubs.Empty) (err error) {
	go func() { time.Sleep(100 * time.Millisecond); os.Exit(0) }()
	return nil
}

// countAliveNeighbours counts the 8 surrounding cells.
// Horizontal edges wrap; vertical neighbours always exist thanks to the halo rows.
func countAliveNeighbours(rows [][]byte, x, y, width int) int {
	count := 0
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue // skip the cell itself
			}
			nx := (x + dx + width) % width // wrap horizontally
			ny := y + dy                   // no vertical wrap needed: halos cover it
			if rows[ny][nx] == 255 {
				count++
			}
		}
	}
	return count
}

// calculateNextState computes rows startY..endY-1 of the (halo-padded) strip.
func calculateNextState(rows [][]byte, startY, endY, width int) [][]byte {
	strip := make([][]byte, endY-startY)
	for i := range strip {
		strip[i] = make([]byte, width)
	}

	for y := startY; y < endY; y++ {
		for x := 0; x < width; x++ {
			n := countAliveNeighbours(rows, x, y, width)
			alive := rows[y][x] == 255
			if (alive && (n == 2 || n == 3)) || (!alive && n == 3) {
				strip[y-startY][x] = 255
			}
		}
	}
	return strip
}

// worker computes one sub-strip and sends it back on its own channel.
func worker(rows [][]byte, startY, endY, width int, out chan<- [][]byte) {
	out <- calculateNextState(rows, startY, endY, width)
}

func main() {
	port := flag.String("port", "8031", "port to listen on")
	threads := flag.Int("threads", runtime.NumCPU(), "goroutines to use inside this worker")
	flag.Parse()

	rpc.Register(&Worker{threads: *threads})
	ln, err := net.Listen("tcp", ":"+*port)
	if err != nil {
		panic(err)
	}
	defer ln.Close()
	rpc.Accept(ln)
}
