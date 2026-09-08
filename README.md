# netso

Secure connectivity **control plane** for peer networks — many networks, strong
per-peer identity, end-to-end (incl. group) encryption, and brokered shell access
to any peer from a browser or CLI.

`Status: Research / Design` · part of [TrustSentinel](https://trustsentinel.eu)

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

## Status
Greenfield — the 2020 prototype was not preserved. This repo starts from the
design in [docs/architecture.md](docs/architecture.md); implementation follows the
roadmap there (Phase 1: minimal Go hub + agent, pairwise Noise, CLI access).

## Recognition
INCIBE National Cybersecurity Competition **2020 — Top 10** (Entrepreneurs track):
a secure decentralized platform built on SSI and E2E encryption.

## License
MIT — see [LICENSE](LICENSE).
