# FlowSteer

FlowSteer is a minimal SD-WAN L4 proxy written in Go. It demonstrates basic TCP
and UDP forwarding with simple load balancing. The proxy exposes a small
management API to update backends and forwarding rules at runtime.

See [docs/PROCESS.md](docs/PROCESS.md) for an overview of how the proxy processes connections.

## Features

- Round-robin balancing across multiple TCP backends.
- UDP forwarding to a single backend.
- Management API to update backend list and five-tuple forwarding rules.
- Automatic health checking of backends.
- Rules can optionally bypass the proxy and connect directly to the original
  destination.
- Propagates original connection information between proxies using the PROXY
  protocol.
- Works with iptables TPROXY redirection to capture the original destination
  using `SO_ORIGINAL_DST`.
- Configuration can be loaded from a JSON file and backends are optional.

## Building

```
go build ./cmd/proxy
```

## Usage

Run the proxy listening on port 8080 with two backend servers:

```
./proxy -listen :8080 -backends 10.0.0.1:9000,10.0.0.2:9000
```

For UDP forwarding:

```
# in a separate program
go run ./cmd/proxy -listen :5353 -backends 127.0.0.1:5354
```

This will forward UDP packets received on port 5353 to the backend.

Backends may be omitted entirely. In that case only rules that do not specify a
backend (or specify `direct`) will cause traffic to be forwarded to the
connection's original destination.

### Configuration File

Startup parameters can also be supplied in a JSON file using the `-config` flag.
Fields mirror the command line options:

```json
{
  "listen": ":8080",
  "backends": ["10.0.0.1:9000", "10.0.0.2:9000"],
  "api": ":9090"
}
```

Launch with:

```bash
./proxy -config config.json
```

## Management API

The proxy starts an HTTP API (default `:9090`) with the following endpoints:

- `PUT /backends` – replace the backend list. Body format:

```json
{ "backends": ["10.0.0.1:9000", "10.0.0.2:9000"] }
```

- `PUT /rules` – replace five-tuple forwarding rules. Example body:

```json
{
  "rules": [
    { "src": "192.168.1.0/24", "dstPort": 8080,
      "proto": "tcp", "backend": "10.0.0.1:9000" },
    { "dst": "10.1.1.0/24", "direct": true }
  ]
}
```

A rule may omit the `backend` field to force the connection to be forwarded
directly to its original destination.

Existing connections are not affected when updating the configuration.

