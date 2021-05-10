package network

// Network is a interface the provides only 2 functions:
// Send: Send anything over the wire.
// Listen: Read anything that is sent to the client.
type Network interface {
	// On successful send, return true.
	Send(any interface{}) bool
	// Return anything heard from the network.
	Listen() interface{}
}
