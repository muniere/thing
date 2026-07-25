package server

import (
	"errors"
	"fmt"
	"net"
	"syscall"
)

// DefaultPort is thingd's default listen port.
const DefaultPort = 4319

// Listen binds a loopback TCP listener on localhost. When explicit is true the
// exact port is used and a bind failure (e.g. the port is taken) is returned
// as-is. Otherwise it starts at port and scans upward for the first free one, so
// several trees can serve at once without the user picking ports by hand. In the
// scan only a taken port is skipped past; any other bind failure (localhost
// unresolvable, permission denied, ...) would repeat for every port, so it is
// returned immediately rather than masked as "no free port".
func Listen(port int, explicit bool) (net.Listener, error) {
	if explicit {
		return net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	}
	const span = 100
	for p := port; p < port+span; p++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", p))
		if err == nil {
			return ln, nil
		}
		if !errors.Is(err, syscall.EADDRINUSE) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("no free port in %d..%d", port, port+span-1)
}
