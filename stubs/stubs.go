package stubs

var EvolveHandler = "GolOperations.Evolve"
var AliveHandler = "GolOperations.AliveCount"

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
