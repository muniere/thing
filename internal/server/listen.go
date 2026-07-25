package server

import (
	"errors"
	"fmt"
	"net"
	"syscall"
)

// DefaultPort is thingd's default listen port.
const DefaultPort = 4319

// Listen binds a TCP listener, matching the tray server's logic. When explicit is
// true the exact port is used and a bind failure is returned as-is; otherwise it
// scans upward from port, advancing to the next port only on EADDRINUSE (any other
// error is returned immediately), so several trees can serve at once without the
// user picking ports by hand.
//
// It binds the wildcard address (":port"), not localhost: "localhost" resolves to
// 127.0.0.1 first, so a localhost bind succeeds even when the port is already held
// on the IPv6 loopback (e.g. another dev server on "[::]:port"), silently putting
// two servers on the "same" port on different families. The wildcard sees the port
// taken on any family.
func Listen(port int, explicit bool) (net.Listener, error) {
	if explicit {
		return net.Listen("tcp", fmt.Sprintf(":%d", port))
	}
	const span = 100
	for p := port; p < port+span; p++ {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", p))
		if err == nil {
			return ln, nil
		}
		if !errors.Is(err, syscall.EADDRINUSE) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("no free port in %d..%d", port, port+span-1)
}
