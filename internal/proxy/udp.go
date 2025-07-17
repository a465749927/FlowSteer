package proxy

import (
	"log"
	"net"
)

// UDPProxy forwards UDP packets to a backend.
type UDPProxy struct {
	backend string
}

// NewUDP creates a UDP proxy to a backend address.
func NewUDP(backend string) *UDPProxy {
	return &UDPProxy{backend: backend}
}

func (p *UDPProxy) Serve(localAddr string) error {
	laddr, err := net.ResolveUDPAddr("udp", localAddr)
	if err != nil {
		return err
	}
	baddr, err := net.ResolveUDPAddr("udp", p.backend)
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		return err
	}
	defer conn.Close()

	buf := make([]byte, 65535)
	for {
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("udp read error: %v", err)
			continue
		}
		_, err = conn.WriteToUDP(buf[:n], baddr)
		if err != nil {
			log.Printf("udp write error: %v", err)
			continue
		}
		// echo response to sender
		_, err = conn.WriteToUDP(buf[:n], addr)
		if err != nil {
			log.Printf("udp echo error: %v", err)
		}
	}
}
