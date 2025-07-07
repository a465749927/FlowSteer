package main

import (
	"encoding/json"
	"flag"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/example/flowsteer/internal/proxy"
)

func main() {
	listen := flag.String("listen", ":8080", "listen address")
	backends := flag.String("backends", "localhost:9000", "comma separated backend addresses")
	apiAddr := flag.String("api", ":9090", "management API address")
	flag.Parse()

	beList := splitAndTrim(*backends)

	p := proxy.New(beList)

	go func() {
		log.Printf("management API listening on %s", *apiAddr)
		if err := serveAPI(p, *apiAddr); err != nil {
			log.Fatalf("api server error: %v", err)
		}
	}()

	l, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	log.Printf("listening on %s, backends=%v", *listen, beList)

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}
		go p.Handle(conn)
	}
}

func splitAndTrim(s string) []string {
	var out []string
	for _, v := range strings.Split(s, ",") {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// serveAPI exposes endpoints to update backends and ACL rules.
func serveAPI(p *proxy.SimpleProxy, addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/backends", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			var req struct {
				Backends []string `json:"backends"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			p.UpdateBackends(req.Backends)
		case http.MethodGet:
			json.NewEncoder(w).Encode(struct {
				Backends []string `json:"backends"`
			}{p.Backends()})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/rules", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			var req struct {
				Rules []ruleReq `json:"rules"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			var rules []proxy.ForwardRule
			for _, rr := range req.Rules {
				rule, err := parseRule(rr)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				rules = append(rules, rule)
			}
			p.UpdateRules(rules)
		case http.MethodGet:
			json.NewEncoder(w).Encode(struct {
				Rules []proxy.ForwardRule `json:"rules"`
			}{p.Rules()})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	return http.ListenAndServe(addr, mux)
}

type ruleReq struct {
	Src     string `json:"src"`
	Dst     string `json:"dst"`
	SrcPort int    `json:"srcPort"`
	DstPort int    `json:"dstPort"`
	Proto   string `json:"proto"`
	Backend string `json:"backend"`
	Direct  bool   `json:"direct"`
}

func parseRule(r ruleReq) (proxy.ForwardRule, error) {
	var fr proxy.ForwardRule
	if r.Src != "" {
		if _, n, err := net.ParseCIDR(r.Src); err == nil {
			fr.SrcNet = n
		} else {
			return fr, err
		}
	}
	if r.Dst != "" {
		if _, n, err := net.ParseCIDR(r.Dst); err == nil {
			fr.DstNet = n
		} else {
			return fr, err
		}
	}
	fr.SrcPort = r.SrcPort
	fr.DstPort = r.DstPort
	fr.Proto = r.Proto
	fr.Backend = r.Backend
	fr.Direct = r.Direct
	return fr, nil
}
