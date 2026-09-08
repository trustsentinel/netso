# netso — architecture

> Status: **design / research**. netso is greenfield (the 2020 INCIBE prototype
> was not preserved). This document is the plan of record; code follows the
> roadmap at the end. Where a component already exists elsewhere in TrustSentinel
> it is reused rather than rebuilt.

## 1. What netso is

netso is a **connectivity control plane** for secure peer networks. It lets you
stand up many independent networks of peers — cloud VMs, on-prem hosts, and
lightweight IoT/edge devices across regions — give each peer a strong identity,
connect them with end-to-end encryption, and reach a shell on any peer from a
browser or CLI. A **hub** (single, or federated/decentralized hubs) coordinates
identity, discovery, policy, and monitoring; it never has to see peer traffic in
the clear.

Think **Tailscale/Teleport, but**: identity is self-sovereign (DID/SSI, netso's
2020 root), the agent is small enough for constrained devices, group/multiparty
encryption is a first-class mode (not just pairwise tunnels), and access +
posture monitoring are built in. Hubs run on your own infra (on-prem or cloud),
single or federated.

### Goals
- **Many networks**, isolated, each with its own peers, policy, and region.
- **Mutual security**: every link is mutually authenticated by peer identity; the
  hub is untrusted for confidentiality (it relays ciphertext, like stk today).
- **Multiparty encryption**: a peer routinely talks to several peers at once —
  as independent pairwise sessions, and (for network-wide/multicast messaging) as
  a genuine encrypted **group**.
- **Autodiscovery** with pluggable strategies (not one hard-wired mechanism).
- **Brokered access**: SSH/Teleport-style shell to any peer from a browser or CLI.
- **Monitoring**: peer/link health plus security posture of the fleet.
- **Small agent**: usable on IoT/edge, not just servers.
- **Deploy anywhere**: hub(s) on-prem or cloud; a single hub or federated hubs.

### Non-goals (initially)
- Being a full VPN replacement on day one (WireGuard data plane is an option, not
  a requirement for v1).
- A public/global coordination service — hubs are operator-run.
- Consensus/blockchain. Trust comes from SSI + policy, not a ledger.

## 2. Positioning

| System | Overlap with netso | Where netso differs |
|---|---|---|
| **Tailscale** | coordination hub + mesh of peers, NAT traversal, relays | self-sovereign identity (not an IdP login), IoT-class agent, group encryption, federated hubs you run |
| **Teleport** | browser/CLI access to hosts, audit | mesh connectivity + IoT + group crypto, not only access |
| **Nebula** | operator-run mesh, CA-based identity, lighthouses | DID/SSI identity + verifiable-credential policy, brokered access, monitoring |
| **WireGuard** | fast encrypted point-to-point transport | WG is only the (optional) data plane; netso is identity + control + access on top |
| **libp2p** | transports, discovery, peer identity | opinionated control plane + access + fleet posture, not a toolkit |

## 3. Concepts

- **Network** — an isolated namespace of peers with its own policy and region(s).
  A peer may belong to more than one network.
- **Peer / node** — anything running the netso agent: a server, a container, or an
  IoT device. Has a stable **identity** (a DID) and a set of network memberships.
- **Hub** — the control plane: peer/network registry, discovery coordination,
  policy distribution, key/credential brokering, monitoring aggregation, and
  relay-of-last-resort. Single or **federated** (hubs replicate/gossip state).
- **Identity** — a DID (self-sovereign). Authorization is expressed with
  verifiable credentials / policy, not a central account.
- **Session** — an authenticated, encrypted channel between peers (pairwise), or
  membership in an encrypted group (multiparty).
- **Access** — a brokered shell/port to a peer, reached from a browser or CLI.

## 4. Architecture

```
                         ┌───────────────────────────────────────────┐
                         │  netso hub (control plane)                  │
    browser / CLI  ─────▶│  registry · discovery · policy · key broker │◀── federate ──▶ hub (other region)
    (access)             │  monitoring · relay-of-last-resort          │
                         └───────────────▲───────────────▲─────────────┘
                                 control  │               │ control
                        ┌────────────────┘               └────────────────┐
                        │                                                  │
                  ┌───────────┐        pairwise E2E (Noise)         ┌───────────┐
                  │  peer A    │◀───────────────────────────────────▶│  peer B   │
                  │  (agent)   │◀───────────────┐          ┌────────▶│  (agent)  │
                  └───────────┘                 │          │         └───────────┘
                        ▲                   ┌───────────┐  │
                        └──── group (MLS) ──│  peer C    │──┘   ← one peer, several
                                            │  (IoT)     │        peers at once
                                            └───────────┘
   Control traffic goes peer↔hub. Data traffic is peer↔peer, end-to-end encrypted;
   the hub relays only when direct connectivity fails, and only ever sees ciphertext.
```

**Control plane (hub).** Peers register, prove their identity, fetch the peer
list + policy for their networks, and publish health. The hub brokers
introductions and short-lived credentials, aggregates monitoring, and relays
traffic when peers cannot connect directly. It is untrusted for confidentiality.

**Data plane (peer ↔ peer).** Direct, end-to-end encrypted links. Default
transport is Noise-framed (as in stk today); WireGuard is an option where a
kernel-fast tunnel is wanted. NAT traversal via STUN/hole-punching, with a
DERP-style relay through the hub as fallback.

**Agent (peer).** Small: identity + discovery client + transport + a policy
cache so it keeps working if the hub is briefly unreachable. A Go agent for
servers/containers; a Rust (`no_std`-capable) agent for constrained IoT.

**Access broker.** Reuses **stk**: a hub-brokered, end-to-end-encrypted shell to
a peer, from a browser (xterm.js + WASM) or a CLI — exactly netso's "SSH or
Teleport from the browser or CLI". Sessions are mutually authenticated and
auditable.

## 5. Identity & mutual security

- Each peer holds a **DID** (candidate methods: `did:key` for devices, `did:web`
  for hub-anchored orgs, `did:peer` for pairwise) with a Noise/ed25519 static key.
- **Mutual authentication** on every link: both ends verify the other's DID
  before a session forms — same property stk's IK handshake gives today
  (initiator pins the responder; responder authorizes the initiator up front).
- **Authorization** via verifiable credentials / policy (which networks, which
  peers, which access), distributed by the hub and cached at the peer.
- **Revocation & rotation**: short-lived credentials; a revoked peer drops out on
  next credential refresh and is removed from group membership.

## 6. Encryption — pairwise vs. multiparty

netso needs two distinct modes; "multiparty encryption" means different things and
netso does both:

**(a) Pairwise (unicast) — the default.** Each peer↔peer link is its own Noise
session (mutual static-key auth, forward secrecy). A peer talking to two peers at
once simply has **two independent Noise sessions**. This is the mesh model
(Tailscale/WireGuard/stk) and covers the large majority of traffic.

**(b) Group (multicast / shared channel) — first-class.** When a message must go
to *a whole network* or a shared channel (config push, telemetry fan-out, a
device group), pairwise re-encryption to N peers is wasteful and leaks the
membership pattern. For that netso uses a **group key** scheme:

- **MLS (RFC 9420)** is the recommended target: asynchronous group key agreement
  with forward secrecy **and** post-compromise security, efficient membership
  changes (add/remove a peer without rekeying pairwise with everyone), and it
  scales to large groups — a good fit for device fleets.
- **Sender Keys** (simpler, Signal-style) is a lighter interim option for small,
  stable groups if MLS is too heavy for the first IoT agent.

**Recommendation:** ship pairwise Noise first (it reuses stk directly), then add
MLS-based groups for network-wide messaging. Constrained devices that can't run
MLS fall back to pairwise or Sender Keys. This is **Open decision #1** below.

## 7. Networks & multi-tenancy
- A **network** is an isolated policy + membership namespace; peers can join
  several. Regions tag peers/hubs for locality and data-residency.
- Per-network policy: who may join, who may reach whom, who may open access.
- Isolation is enforced at the hub (registry/policy) and at the peer (a peer only
  holds keys/credentials for networks it belongs to).

## 8. Autodiscovery (pluggable strategies)
A peer finds its peers via a **strategy interface**, chosen per network/environment:
- **Hub-coordinated** (default) — register with the hub, receive the peer list.
- **Local** — mDNS / DNS-SD for same-LAN IoT.
- **Decentralized** — DHT / gossip for hub-optional operation (ties into argos'
  P2P research and kuipeers' discovery work).
- **Rendezvous / bootstrap** — seed nodes for cold start behind NAT.
- **Cloud** — provider metadata/tag-based enumeration.
- **Static** — explicit peer list for locked-down deployments.

## 9. Connectivity & NAT traversal
Direct first (public or hole-punched via STUN), then a **relay through the hub**
(DERP-style) as fallback. Relayed traffic stays end-to-end encrypted — the hub
forwards ciphertext.

## 10. Monitoring
- **Liveness/health** — peer heartbeat, link state, latency, relay usage.
- **Posture** — reuse **argos**: location/ASN/software and **vulnerability
  posture** of each peer, so the fleet's security state is visible.
- **Audit** — every brokered access session is recorded (who, which peer, when).
- **Telemetry** — OpenTelemetry traces/metrics from hub and agents.

## 11. Deployment topologies
- **Single hub** — one coordinator (on-prem or cloud); simplest; HA via replicas.
- **Federated hubs** — per-region/per-org hubs that replicate registry + policy
  and relay across the federation; no single global authority. This is the
  "decentralized hubs" model.
- **Edge hub** — a small hub co-located with an IoT region for local coordination
  and offline tolerance, syncing upward when connected.

## 12. How netso composes existing TrustSentinel projects
netso is largely an **integration + control-plane** effort — much of the hard
security machinery already exists:

| netso capability | Reuses |
|---|---|
| Brokered browser/CLI shell to peers | **stk** (hub relay + Noise + xterm/WASM + CLI) |
| Encrypted mesh transport, Noise agent | **marshmallows** (secure mesh, Tailscale-like) |
| Stealth — peers expose no open port until authenticated | **stuk** (port-knocking / gated access) |
| Fleet posture / vulnerability view, P2P discovery research | **argos** (+ kuipeers) |
| Peer identity handshake (mutual, early-auth) | **stk `internal/secure`** (Noise IK) |

## 13. Roadmap (phased, greenfield)
- **Phase 0 — design** *(this doc)*. Pin the open decisions below.
- **Phase 1 — minimal hub + agent.** Go hub (registry + hub-coordinated discovery
  + relay) and Go agent; **pairwise Noise** links (reuse stk's `internal/secure`);
  **CLI access** to a peer via the hub. One network. `kind`/compose demo + CI,
  mirroring stk/stuk.
- **Phase 2 — access + multi-network + monitoring.** Browser access (stk WASM
  client), several isolated networks, health monitoring, audit of sessions.
- **Phase 3 — SSI identity.** DID method + verifiable-credential authorization;
  revocation/rotation.
- **Phase 4 — groups + federation + IoT.** MLS-based group encryption for
  network-wide messaging; federated hubs; a Rust (`no_std`) IoT agent.
- **Phase 5 — stealth + posture.** Integrate stuk (no open ports until
  authenticated) and argos (fleet vulnerability posture).

## 14. Open design decisions (need a steer)
1. **Multiparty model** — pairwise-only first, then add **MLS** groups? (recommended)
   Or invest in group crypto earlier? Affects the IoT agent's minimum footprint.
2. **Control plane** — start **centralized single hub** (design for federation) or
   build federation from day one? (recommend: centralized first, federation-ready
   data model.)
3. **Data plane** — **Noise-framed** (reuse stk) as default, WireGuard as an
   option? Or WireGuard-first like Tailscale? (recommend: Noise-first, WG later.)
4. **DID method(s)** — `did:key` (devices) + `did:web` (orgs) to start?
5. **Languages** — **Go** for hub + server agent, **Rust** for the IoT agent?
6. **Group scope** — is network-wide multicast actually needed for the first use
   cases, or is pairwise enough until Phase 4?

## 15. Threat model (summary)
- **Hub is untrusted for confidentiality** — it coordinates and relays ciphertext;
  compromise exposes metadata/policy, not session plaintext.
- **Peer compromise** — contained by per-network key isolation, short-lived
  credentials, and (for groups) MLS post-compromise security + removal.
- **Network exposure** — stuk-style gating means peers can expose no inbound port
  until a peer is authenticated.
- **Replay / MITM** — defeated by mutual DID authentication and Noise's handshake
  (pinning + fresh ephemerals), as in stk today.
