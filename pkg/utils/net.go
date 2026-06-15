package utils

import (
	"fmt"
	"net"
	"strconv"
)

// ExtractHostPort parses an address string (host:port, :port, host) and returns host, port, and error.
func ExtractHostPort(addr string) (host string, port int, err error) {
	if addr == "" {
		return "", 0, fmt.Errorf("empty host")
	}
	host, portStr, splitErr := net.SplitHostPort(addr)
	if splitErr == nil {
		p, atoiErr := strconv.Atoi(portStr)
		if atoiErr == nil {
			return host, p, nil
		}
		return host, 0, atoiErr
	}
	// If addr is just a port (e.g., ":8080"), host will be ""
	if len(addr) > 0 && addr[0] == ':' {
		p, atoiErr := strconv.Atoi(addr[1:])
		if atoiErr == nil {
			return "", p, nil
		}
		return "", 0, atoiErr
	}
	// If addr is just a host (no port)
	return addr, 0, nil
}
