package proxy

import (
	"io"
	"log"
	"net"
	"strconv"
	"sync/atomic"
	"time"
)

// SimpleProxy forwards TCP connections to a list of backend servers with round-robin balancing.
// SimpleProxy forwards TCP connections to backend servers. The backend list and
// forwarding rules can be updated at runtime.
type backend struct {
	addr  string
	alive atomic.Bool
}

// connInfo describes a connection with its original five-tuple.
type connInfo struct {
	net.Conn
	srcIP     net.IP
	srcPort   int
	dstIP     net.IP
	dstPort   int
	origDst   string
	fromProxy bool
}

// parseConn extracts connection information and handles the PROXY protocol.
func parseConn(c net.Conn) (*connInfo, error) {
	conn, src, dst, err := parseProxyHeader(c)
	if err != nil {
		return nil, err
	}

	ci := &connInfo{Conn: conn}
	if src != "" && dst != "" {
		ci.fromProxy = true
		if host, portStr, err := net.SplitHostPort(src); err == nil {
			ci.srcIP = net.ParseIP(host)
			p, _ := strconv.Atoi(portStr)
			ci.srcPort = p
		}
		if host, portStr, err := net.SplitHostPort(dst); err == nil {
			ci.dstIP = net.ParseIP(host)
			p, _ := strconv.Atoi(portStr)
			ci.dstPort = p
			ci.origDst = dst
		}
		return ci, nil
	}

	raddr, rok := c.RemoteAddr().(*net.TCPAddr)
	laddr, lok := c.LocalAddr().(*net.TCPAddr)
	if rok {
		ci.srcIP = raddr.IP
		ci.srcPort = raddr.Port
	}
	if lok {
		ci.dstIP = laddr.IP
		ci.dstPort = laddr.Port
	}
	return ci, nil
}

// SimpleProxy forwards TCP connections to backend servers. The backend list and
// forwarding rules can be updated at runtime. Backend health is probed
// periodically and only healthy backends are selected when possible.
type SimpleProxy struct {
	backends atomic.Value // []*backend
	rules    atomic.Value // []ForwardRule
	current  uint32
	interval time.Duration
}

// New creates a new proxy with the given backend addresses.
// New creates a new proxy with the given backend addresses.
func New(backends []string) *SimpleProxy {
	p := &SimpleProxy{interval: 5 * time.Second}
	p.UpdateBackends(backends)
	p.rules.Store([]ForwardRule{})
	go p.healthLoop()
	return p
}

// UpdateBackends replaces the backend list used for new connections.
func (p *SimpleProxy) UpdateBackends(b []string) {
	var out []*backend
	for _, addr := range b {
		if addr == "" {
			continue
		}
		be := &backend{addr: addr}
		be.alive.Store(true)
		out = append(out, be)
	}
	p.backends.Store(out)
}

// Backends returns the current backend list.
func (p *SimpleProxy) Backends() []string {
	bs, _ := p.backends.Load().([]*backend)
	out := make([]string, 0, len(bs))
	for _, b := range bs {
		out = append(out, b.addr)
	}
	return out
}

// UpdateRules replaces the forwarding rules.
func (p *SimpleProxy) UpdateRules(r []ForwardRule) {
	p.rules.Store(append([]ForwardRule(nil), r...))
}

// Rules returns the current rules.
func (p *SimpleProxy) Rules() []ForwardRule {
	r, _ := p.rules.Load().([]ForwardRule)
	out := make([]ForwardRule, len(r))
	copy(out, r)
	return out
}

// nextBackend returns the next backend address using round robin.
func (p *SimpleProxy) nextBackend() *backend {
	bs, _ := p.backends.Load().([]*backend)
	if len(bs) == 0 {
		return nil
	}
	start := atomic.AddUint32(&p.current, 1)
	for i := 0; i < len(bs); i++ {
		b := bs[int(start+uint32(i))%len(bs)]
		if b.alive.Load() {
			return b
		}
	}
	return bs[int(start)%len(bs)]
}

func (p *SimpleProxy) selectByRules(srcIP net.IP, srcPort int, dstIP net.IP, dstPort int) (string, bool, bool) {
	rules, _ := p.rules.Load().([]ForwardRule)
	if len(rules) == 0 {
		return "", false, false
	}
	for _, r := range rules {
		if r.Match(srcIP, srcPort, dstIP, dstPort, "tcp") {
			return r.Backend, r.Direct, true
		}
	}
	return "", false, false
}

// Handle starts a new goroutine to proxy the connection to a backend.
func (p *SimpleProxy) Handle(client net.Conn) {
	ci, err := parseConn(client)
	if err != nil {
		log.Printf("failed to parse connection: %v", err)
		client.Close()
		return
	}

	backendAddr, direct, ok := p.selectByRules(ci.srcIP, ci.srcPort, ci.dstIP, ci.dstPort)
	if direct {
		p.directForward(ci)
		return
	}
	if !ok {
		be := p.nextBackend()
		if be == nil {
			log.Printf("no backend available")
			ci.Close()
			return
		}
		backendAddr = be.addr
	}

	backend, err := net.Dial("tcp", backendAddr)
	if err != nil {
		log.Printf("failed to connect to backend %s: %v", backendAddr, err)
		ci.Close()
		return
	}

	// propagate original tuple to next proxy
	if err := sendProxyHeader(backend, ci.srcIP, ci.srcPort, ci.dstIP, ci.dstPort); err != nil {
		log.Printf("failed to send proxy header: %v", err)
		ci.Close()
		backend.Close()
		return
	}

	go proxyConn(ci, backend)
	go proxyConn(backend, ci)
}

func proxyConn(src, dst net.Conn) {
	defer src.Close()
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		log.Printf("proxy error: %v", err)
	}
}

// directForward forwards the connection to its original destination using
// SO_ORIGINAL_DST. If the destination cannot be determined the connection is
// closed.
func (p *SimpleProxy) directForward(ci *connInfo) {
	dst := ci.origDst
	if dst == "" {
		tcp, ok := ci.Conn.(*net.TCPConn)
		if !ok {
			ci.Close()
			return
		}
		var err error
		dst, err = originalDst(tcp)
		if err != nil {
			log.Printf("failed to get original dst: %v", err)
			ci.Close()
			return
		}
	}

	backend, err := net.Dial("tcp", dst)
	if err != nil {
		log.Printf("failed to dial origin %s: %v", dst, err)
		ci.Close()
		return
	}
	// propagate tuple when direct-forwarding through another proxy
	if err := sendProxyHeader(backend, ci.srcIP, ci.srcPort, ci.dstIP, ci.dstPort); err != nil {
		log.Printf("failed to send proxy header: %v", err)
		ci.Close()
		backend.Close()
		return
	}
	go proxyConn(ci, backend)
	go proxyConn(backend, ci)
}

// healthLoop periodically checks backend reachability.
func (p *SimpleProxy) healthLoop() {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for range ticker.C {
		bs, _ := p.backends.Load().([]*backend)
		for _, b := range bs {
			conn, err := net.DialTimeout("tcp", b.addr, 2*time.Second)
			if err != nil {
				b.alive.Store(false)
				continue
			}
			conn.Close()
			b.alive.Store(true)
		}
	}
}
