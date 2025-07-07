# FlowSteer Code Processing Flow

This document describes how the proxy handles connections and how the main components interact.

## Startup

- `cmd/proxy/main.go` parses command-line flags to obtain the listening address, the initial list of backends and the API address.
- A TCP listener is created with `IP_TRANSPARENT` enabled so the proxy can accept traffic redirected via iptables TPROXY.
- The management API is started in a goroutine and exposes `/backends` and `/rules` for runtime configuration.

## Accepting Connections

- The main loop accepts incoming TCP connections and hands each connection to `SimpleProxy.Handle`.

## Parsing Connection Information

- `Handle` calls `parseConn` which reads an optional PROXY protocol header.
- If a header is present, the original source and destination addresses are taken from it.
- Otherwise the connection's local and remote addresses are used and `SO_ORIGINAL_DST` is queried on Linux to recover the original destination of TPROXY traffic.
- The resulting `connInfo` contains the full five‑tuple used for rule selection and forwarding.

## Rule Evaluation

- `selectByRules` scans the configured forwarding rules (`ForwardRule` in `rule.go`).
- A rule can specify source/destination networks, ports and protocol; it may also indicate `Direct` forwarding which bypasses load balancing and connects to the original destination.

## Forwarding

1. **Direct Mode**
   - If a rule with `Direct` is matched, `directForward` is called.
   - The connection is opened to the original destination without sending a
     PROXY header.
   - Data is proxied bidirectionally using `io.Copy`.

2. **Backend Mode**
   - Otherwise `nextBackend` selects a healthy backend in round‑robin fashion.
   - A TCP connection is opened to that backend and a PROXY header describing the client tuple is sent.
   - Two goroutines copy data between client and backend.

## Health Checking

- `healthLoop` periodically dials each backend (every five seconds by default).
- Backends that fail the probe are marked unhealthy and skipped by `nextBackend` until they recover.

## Management API

- `serveAPI` (in `main.go`) listens on the configured API address.
- `PUT /backends` replaces the backend list.
- `PUT /rules` replaces the forwarding rules.
- Existing connections continue using their previously selected backend; updates only affect new connections.

## UDP Forwarding

- `UDPProxy` in `udp.go` listens on a UDP socket and forwards packets to a single backend.
- Responses from the backend are echoed back to the original sender.

