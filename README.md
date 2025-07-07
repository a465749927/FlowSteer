# FlowSteer

FlowSteer is a minimal SD-WAN L4 proxy written in Go. It demonstrates basic TCP
and UDP forwarding with simple load balancing. The proxy exposes a small
management API to update backends and forwarding rules at runtime.

## Features

- Round-robin balancing across multiple TCP backends.
- UDP forwarding to a single backend.
- Management API to update backend list and five-tuple forwarding rules.
- Automatic health checking of backends.
- Rules can optionally bypass the proxy and connect directly to the original
  destination.

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

Existing connections are not affected when updating the configuration.

