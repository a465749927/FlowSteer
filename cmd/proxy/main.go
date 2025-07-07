package main

import (
	"flag"
	"log"
	"net"

	"flowsteer/internal/proxy"
)

func main() {
	backend := flag.String("backend", "localhost:8080", "backend address")
	enableProxyProtocol := flag.Bool("proxy-protocol", false, "enable PROXY protocol forwarding")
	addr := flag.String("listen", ":8080", "listen address")
	flag.Parse()

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	p := &proxy.SimpleProxy{BackendAddr: *backend, EnableProxyProtocol: *enableProxyProtocol}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Print(err)
			continue
		}
		go p.Handle(conn)
	}
}
