package stubs

var EvolveHandler = "GolOperations.Evolve"

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
