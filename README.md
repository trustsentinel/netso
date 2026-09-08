# netso

Secure connectivity **control plane** for peer networks — many networks, strong
per-peer identity, end-to-end (incl. group) encryption, and brokered shell access
to any peer from a browser or CLI.

[![CI](https://github.com/trustsentinel/netso/actions/workflows/ci.yml/badge.svg)](https://github.com/trustsentinel/netso/actions/workflows/ci.yml)
`Status: Phase 2 (Go)` · `hub/agent + CLI & browser access + monitoring` · part of [TrustSentinel](https://trustsentinel.eu)

## Overview
netso connects heterogeneous peers — cloud VMs, on-prem hosts, and lightweight
IoT/edge devices across regions — into isolated, mutually-authenticated networks.
A **hub** (single, or federated/decentralized hubs on your own infra) coordinates
identity, discovery, policy, and monitoring, and relays only when peers can't
connect directly — it never needs to see traffic in the clear.

Think **Tailscale/Teleport, but** with self-sovereign identity (DID/SSI — netso's
2020 root), an agent small enough for constrained devices, **multiparty/group
encryption** as a first-class mode, and access + posture monitoring built in.

## Features (planned)
- **Many networks** — isolated, per-network policy, region-aware.
- **Autodiscovery** — pluggable strategies (hub-coordinated, mDNS, DHT/gossip,
  rendezvous, cloud metadata, static).
- **Mutual security** — every link mutually authenticated by peer identity; the
  hub is untrusted for confidentiality.
- **Multiparty encryption** — pairwise Noise sessions, plus MLS-based groups for
  network-wide messaging (a peer talks to many peers at once).
- **Brokered access** — SSH/Teleport-style shell to any peer from a browser or CLI.
- **Monitoring** — peer/link health and fleet **security posture**.

## Architecture
Design of record: **[docs/architecture.md](docs/architecture.md)** — components,
the encryption model (pairwise vs. group/MLS), discovery, deployment topologies,
threat model, a phased roadmap, and the open design decisions.

netso is largely a control-plane + integration effort over existing TrustSentinel
building blocks:

| Capability | Reuses |
|---|---|
| Brokered browser/CLI shell to peers | [stk](https://github.com/trustsentinel/stk) |
| Encrypted mesh transport / Noise agent | [marshmallows](https://github.com/trustsentinel/marshmallows) |
| Stealth (no open port until authenticated) | [stuk](https://github.com/trustsentinel/stuk) |
| Fleet posture + P2P discovery research | [argos](https://github.com/trustsentinel/argos) |

## Quick start
Implemented: a Go hub + agent + CLI, pairwise mutually-authenticated Noise,
hub-coordinated discovery, a brokered shell to a peer from the **CLI or a
browser**, and a monitoring endpoint.

```bash
# containerized end-to-end demo (hub + two peers + a client)
cd deploy/compose && docker compose build && docker compose run --rm e2e && docker compose down -v

# or run it locally without Docker
./smoke.sh
```

CLI:
```bash
netso keygen -identity ~/.netso/id                          # a client identity
netso-hub -addr :8443                                       # the hub
netso-agent -hub http://hub:8443 -network prod -name web \  # a peer
            -authorized-clients clients.txt
netso peers -hub http://hub:8443 -network prod              # discover peers
netso ssh   -hub http://hub:8443 -network prod -peer web \  # brokered shell
            -identity ~/.netso/id
```

Browser access — the same Go Noise client compiled to WebAssembly + xterm.js:
```bash
make web                    # build web/netso.wasm
./browser-demo.sh           # hub (serving web/) + two peers; prints a URL
```
Open the URL, **List peers**, pick one, **Connect** — a shell in the browser. Serve
it in production with `netso-hub -webdir web`.

Monitoring: `GET /status` returns peers per network, live session count, and a
recent-session audit log.

## Layout
- `cmd/netso-hub` — control plane: registry, discovery (`/peers`), relay (`/connect`), monitoring (`/status`), serves the browser client
- `cmd/netso-agent` — peer agent: dials out, registers, serves a PTY shell over Noise
- `cmd/netso` — client CLI: `keygen`, `peers`, `ssh`
- `cmd/netso-wasm` + `web/` — the browser client (Go→WebAssembly + xterm.js)
- `internal/secure` — Noise **IK** mutual-auth session + identities + enrollment (shared lineage with stk)
- `internal/registry` — networks + peers · `internal/audit` — session monitoring log
- `internal/transport` — message-framed connection (websocket / browser / pipe) · `internal/shell` — PTY

## Status
Phases 1–2 build, are unit-tested (`-race`), verified in a real browser, have a
runnable Compose e2e, and GitHub Actions CI. Done so far: hub/agent/CLI, pairwise
Noise IK + enrollment, discovery, **CLI and browser access**, and **monitoring**
(`/status`). The greenfield 2020 prototype was not preserved; later phases —
SSI/DID identity, MLS group encryption, federated hubs, IoT agent, stealth
(stuk) + fleet posture (argos) — follow the roadmap in
[docs/architecture.md](docs/architecture.md).

## Recognition
INCIBE National Cybersecurity Competition **2020 — Top 10** (Entrepreneurs track):
a secure decentralized platform built on SSI and E2E encryption.

## License
MIT — see [LICENSE](LICENSE).
