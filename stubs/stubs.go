package stubs

var EvolveHandler = "GolOperations.Evolve"
var AliveHandler = "GolOperations.AliveCount"
var GetWorldHandler = "GolOperations.GetWorld"
var PauseHandler = "GolOperations.Pause"
var QuitHandler = "GolOperations.Quit"
var KillHandler = "GolOperations.Kill"

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
