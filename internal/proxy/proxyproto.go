package proxy

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

// parseProxyHeader checks for a HAProxy PROXY protocol v1 header on conn.
// If found, it returns the source and destination addresses from the header
// and a wrapped net.Conn that will supply the remaining data.
func parseProxyHeader(conn net.Conn) (net.Conn, string, string, error) {
	r := bufio.NewReader(conn)
	peek, err := r.Peek(6)
	if err == nil && string(peek) == "PROXY " {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, "", "", err
		}
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) != 6 {
			return nil, "", "", fmt.Errorf("invalid PROXY header")
		}
		srcIP := fields[2]
		dstIP := fields[3]
		srcPort := fields[4]
		dstPort := fields[5]
		return &connReader{Conn: conn, r: r}, net.JoinHostPort(srcIP, srcPort), net.JoinHostPort(dstIP, dstPort), nil
	}
	return &connReader{Conn: conn, r: r}, "", "", nil
}

// sendProxyHeader writes a PROXY protocol v1 header to conn describing the given tuple.
func sendProxyHeader(conn net.Conn, srcIP net.IP, srcPort int, dstIP net.IP, dstPort int) error {
	proto := "TCP4"
	if srcIP.To4() == nil || dstIP.To4() == nil {
		proto = "TCP6"
	}
	header := fmt.Sprintf("PROXY %s %s %s %d %d\r\n", proto, srcIP.String(), dstIP.String(), srcPort, dstPort)
	_, err := conn.Write([]byte(header))
	return err
}

// connReader wraps a net.Conn with a bufio.Reader used to put bytes back after peeking.
type connReader struct {
	net.Conn
	r *bufio.Reader
}

func (c *connReader) Read(b []byte) (int, error) {
	return c.r.Read(b)
}
