package proxy

import (
	"net"
	"strings"
)

// ForwardRule associates a 5-tuple with a backend address.
type ForwardRule struct {
	SrcNet  *net.IPNet `json:"src,omitempty"`
	DstNet  *net.IPNet `json:"dst,omitempty"`
	SrcPort int        `json:"srcPort,omitempty"`
	DstPort int        `json:"dstPort,omitempty"`
	Proto   string     `json:"proto,omitempty"` // tcp or udp
	Backend string     `json:"backend,omitempty"`
	Direct  bool       `json:"direct,omitempty"`
}

// Match reports whether the rule matches the provided tuple.
func (r ForwardRule) Match(srcIP net.IP, srcPort int, dstIP net.IP, dstPort int, proto string) bool {
	if r.Proto != "" && !strings.EqualFold(r.Proto, proto) {
		return false
	}
	if r.SrcNet != nil && (srcIP == nil || !r.SrcNet.Contains(srcIP)) {
		return false
	}
	if r.DstNet != nil && (dstIP == nil || !r.DstNet.Contains(dstIP)) {
		return false
	}
	if r.SrcPort != 0 && r.SrcPort != srcPort {
		return false
	}
	if r.DstPort != 0 && r.DstPort != dstPort {
		return false
	}
	return true
}

// ACL holds forwarding rules.
type ACL struct {
	Rules []ForwardRule
}

// SelectBackend returns the backend for the tuple if a rule matches.
func (a *ACL) SelectBackend(srcIP net.IP, srcPort int, dstIP net.IP, dstPort int, proto string) (string, bool) {
	for _, r := range a.Rules {
		if r.Match(srcIP, srcPort, dstIP, dstPort, proto) {
			return r.Backend, true
		}
	}
	return "", false
}
