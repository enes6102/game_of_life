package gol

import "flag"

// brokerAddr is the broker the local controller connects to.
// Override with -broker, e.g. `go run . -broker 54.89.14.29:8030`
// or `go test ./tests -run TestAlive -args -broker 54.89.14.29:8030`.
var brokerAddr = flag.String("broker", "127.0.0.1:8030", "IP:port of the GoL broker")
