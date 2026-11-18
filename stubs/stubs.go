package stubs

var EvolveHandler = "GolOperations.Evolve"
var AliveHandler = "GolOperations.AliveCount"
var GetWorldHandler = "GolOperations.GetWorld"
var PauseHandler = "GolOperations.Pause"
var QuitHandler = "GolOperations.Quit"
var KillHandler = "GolOperations.Kill"
var WorkerUpdateHandler = "Worker.Update"
var WorkerKillHandler = "Worker.Kill"

type WorkerRequest struct {
	Rows  [][]byte // halo above + strip + halo below
	Width int
}

type WorkerResponse struct {
	Strip [][]byte // just the strip len(Rows)-2 rows
}

type Empty struct{}

type WorldResponse struct {
	World [][]byte
	Turn  int
}

type PauseResponse struct {
	Paused bool
	Turn   int
}

type AliveRequest struct{}

type AliveResponse struct {
	CompletedTurns int
	CellsCount     int
}

type Request struct {
	World  [][]byte
	Width  int
	Height int
	Turns  int
}

type Response struct {
	World          [][]byte
	CompletedTurns int
}
