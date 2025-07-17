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


## Redirecting traffic with iptables TPROXY

FlowSteer can operate as a transparent proxy by using iptables to intercept TCP or UDP traffic. Start the proxy normally (for example `sudo ./proxy -listen :8080` for TCP). Then configure the system to redirect connections:

```bash
# enable forwarding and load kernel modules
sudo sysctl -w net.ipv4.ip_forward=1
sudo modprobe xt_TPROXY nf_tproxy_ipv4

# mark packets so they are routed back to the local machine
sudo ip rule add fwmark 1 lookup 100
sudo ip route add local 0.0.0.0/0 dev lo table 100

# redirect TCP traffic to the proxy
sudo iptables -t mangle -N FLOWSTEER
sudo iptables -t mangle -A PREROUTING -p tcp -j FLOWSTEER
sudo iptables -t mangle -A FLOWSTEER -p tcp -j TPROXY --on-port 8080 --tproxy-mark 1

# redirect UDP (example for DNS)
sudo iptables -t mangle -A FLOWSTEER -p udp --dport 53 -j TPROXY --on-port 5353 --tproxy-mark 1
```

With these rules any matching packets are transparently forwarded through FlowSteer while their original destination is preserved.

## NAT Traversal Deployment

Running FlowSteer on both sides of a NAT allows traffic to cross network boundaries without direct port forwarding. A typical layout looks like:

```
client -> Public FlowSteer <=====> FlowSteer inside LAN -> internal service
```

1. **Public instance**: start FlowSteer on a host with a public IP.
   ```bash
   ./proxy -listen :9000 -api :9090
   ```
2. **Internal instance**: start another FlowSteer behind the NAT and configure it to use the public instance as its backend.
   ```bash
   ./proxy -listen :8000 -backends public.ip.address:9000 -api :9091
   ```
   Use iptables TPROXY on the internal gateway to send selected traffic to the internal instance.
3. Traffic arriving at the public proxy is forwarded to the internal instance using the PROXY protocol which carries the original five‑tuple. The internal proxy connects to the final destination on the private network, achieving a simple form of NAT traversal.

