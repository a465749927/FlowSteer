package proxy

import (
	"io"
	"log"
	"net"
	"sync/atomic"
)

// SimpleProxy forwards TCP connections to a list of backend servers with round-robin balancing.
// SimpleProxy forwards TCP connections to backend servers. The backend list and
// forwarding rules can be updated at runtime.
type SimpleProxy struct {
	backends atomic.Value // []string
	rules    atomic.Value // []ForwardRule
	current  uint32
}

// New creates a new proxy with the given backend addresses.
// New creates a new proxy with the given backend addresses.
func New(backends []string) *SimpleProxy {
	p := &SimpleProxy{}
	p.backends.Store(backends)
	p.rules.Store([]ForwardRule{})
	return p
}

// UpdateBackends replaces the backend list used for new connections.
func (p *SimpleProxy) UpdateBackends(b []string) {
	p.backends.Store(append([]string(nil), b...))
}

// Backends returns the current backend list.
func (p *SimpleProxy) Backends() []string {
	b, _ := p.backends.Load().([]string)
	out := make([]string, len(b))
	copy(out, b)
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
func (p *SimpleProxy) nextBackend() string {
	b, _ := p.backends.Load().([]string)
	if len(b) == 0 {
		return ""
	}
	idx := atomic.AddUint32(&p.current, 1)
	return b[int(idx)%len(b)]
}

func (p *SimpleProxy) selectByRules(c net.Conn) (string, bool) {
	rules, _ := p.rules.Load().([]ForwardRule)
	if len(rules) == 0 {
		return "", false
	}
	laddr, lok := c.LocalAddr().(*net.TCPAddr)
	raddr, rok := c.RemoteAddr().(*net.TCPAddr)
	if !lok || !rok {
		return "", false
	}
	for _, r := range rules {
		if r.Match(raddr.IP, raddr.Port, laddr.IP, laddr.Port, "tcp") {
			return r.Backend, true
		}
	}
	return "", false
}

// Handle starts a new goroutine to proxy the connection to a backend.
func (p *SimpleProxy) Handle(client net.Conn) {
	backendAddr, ok := p.selectByRules(client)
	if !ok {
		backendAddr = p.nextBackend()
	}
	if backendAddr == "" {
		log.Printf("no backend available")
		client.Close()
		return
	}

	backend, err := net.Dial("tcp", backendAddr)
	if err != nil {
		log.Printf("failed to connect to backend %s: %v", backendAddr, err)
		client.Close()
		return
	}

	go proxyConn(client, backend)
	go proxyConn(backend, client)
}

func proxyConn(src, dst net.Conn) {
	defer src.Close()
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		log.Printf("proxy error: %v", err)
	}
}
