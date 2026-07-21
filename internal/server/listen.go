package server

import (
	"fmt"
	"net"
)

// DefaultPort is thingd's default listen port.
const DefaultPort = 4319

// Listen binds a loopback TCP listener. When explicit is true the exact port is
// used and a bind failure (e.g. the port is taken) is returned as-is. Otherwise
// it starts at port and scans upward for the first free one, so several trees
// can serve at once without the user picking ports by hand.
func Listen(port int, explicit bool) (net.Listener, error) {
	if explicit {
		return net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	}
	const span = 100
	for p := port; p < port+span; p++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			return ln, nil
		}
	}
	return nil, fmt.Errorf("no free port in %d..%d", port, port+span-1)
}
