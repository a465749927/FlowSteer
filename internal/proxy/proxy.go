package proxy

import (
	"bufio"
	"fmt"
	"io"
	"net"
)

// SimpleProxy forwards connections to a backend address.
type SimpleProxy struct {
	BackendAddr         string
	EnableProxyProtocol bool
}

// Handle accepts a client connection and forwards it to the backend.
func (p *SimpleProxy) Handle(client net.Conn) {
	defer client.Close()

	backend, err := net.Dial("tcp", p.BackendAddr)
	if err != nil {
		return
	}
	defer backend.Close()

	if p.EnableProxyProtocol {
		sendProxyHeader(client, backend)
	}

	go io.Copy(backend, client)
	io.Copy(client, backend)
}

// sendProxyHeader writes a minimal PROXY protocol v1 header to backend.
func sendProxyHeader(client net.Conn, backend net.Conn) {
	caddr, _ := addrParts(client.RemoteAddr())
	baddr, _ := addrParts(backend.LocalAddr())
	header := fmt.Sprintf("PROXY TCP4 %s %s %d %d\r\n", caddr.IP, baddr.IP, caddr.Port, baddr.Port)
	backend.Write([]byte(header))
}

// addr holds IP and port parts of an address.
type addr struct {
	IP   string
	Port int
}

func addrParts(a net.Addr) (addr, error) {
	tcpAddr, ok := a.(*net.TCPAddr)
	if !ok {
		return addr{}, fmt.Errorf("not TCP addr")
	}
	return addr{IP: tcpAddr.IP.String(), Port: tcpAddr.Port}, nil
}

// ParseProxyHeader parses a PROXY protocol v1 header from r and returns the source addr.
func ParseProxyHeader(r *bufio.Reader) (net.Addr, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if len(line) < 6 || line[:5] != "PROXY" {
		return nil, fmt.Errorf("missing PROXY header")
	}
	var proto, srcIP, dstIP string
	var srcPort, dstPort int
	if _, err := fmt.Sscanf(line, "PROXY %s %s %s %d %d", &proto, &srcIP, &dstIP, &srcPort, &dstPort); err != nil {
		return nil, err
	}
	return &net.TCPAddr{IP: net.ParseIP(srcIP), Port: srcPort}, nil
}
