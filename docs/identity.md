# netso — identity (SSI + DIDs)

> Phase 3 of [architecture.md](architecture.md). This is netso's original reason
> for being — the INCIBE 2020 project was a **self-sovereign identity (SSI) + E2E**
> platform. Here that becomes concrete: every device gets a **DID**, anchored in a
> **blockchain registry**, so devices identify and trust each other with no central
> authority.

## Goal
Give each device a **self-sovereign identity** it fully controls, that any hub or
peer can **verify and check for revocation** without asking a central server —
using W3C **DIDs** with a **blockchain** as the decentralized registry.

## The key idea: identity *is* the transport key
A device's DID is derived deterministically from its **Noise/X25519 static key**
(the key it already authenticates the transport with) as a **`did:key`**:

```
did:key:z6LS…   =  did:key: + multibase-base58btc( x25519-multicodec || pubkey )
```

So there is no separate signing identity and no CA: a peer **proves control of its
DID by completing the Noise handshake** with the key the DID commits to. netso's
existing mutual-auth handshake already does this — DIDs add a *name* and a
*registry* on top of it.

## What the blockchain is for (and isn't)
The chain is a **Verifiable Data Registry (VDR)** — the decentralized root of
trust, not the traffic path:

- **Anchor** each device's DID document (its key + metadata) and its network
  authorizations.
- **Resolve** a DID → document from anywhere, with no central directory.
- **Revoke** a device (registry status update) so it drops out fleet-wide.

It is explicitly **not** on the hot path: devices never touch the chain to *use*
their identity, and hubs keep an **off-chain resolver cache** so resolution and
verification work at the edge and while offline. This matters for IoT — a
constrained device only has to *hold a key*; anchoring and resolution are done for
it by an enrollment service / hub.

## Roles (SSI triangle)
- **Holder** — the device. Self-issues its `did:key` from its own key; holds it.
- **Issuer / registry** — the enrollment service writes the DID document and
  authorization to the blockchain VDR on the device's behalf.
- **Verifier** — a hub or peer. Resolves the DID, checks the presented handshake
  key matches, and checks revocation — then allows the session.

## Lifecycle
1. **Generate** — device creates its Noise keypair → `did:key` (offline, free).
2. **Enroll / anchor** — register the DID document + network membership in the
   registry (on-chain). For IoT this is done by the enrolment service.
3. **Discover** — the hub advertises peers by **DID** (Phase 1/2 discovery returns
   the key today; it returns the DID here).
4. **Verify** — on connect, the peer's handshake key must match the DID
   (`did.Matches`) **and** the registry must show it registered and not revoked
   (`did.Authorized`). Only then does the shell open.
5. **Revoke / rotate** — a registry status update removes the device on the next
   resolve or cache refresh.

## DID method — the decision (open)
The `did:key` above is the self-certifying *device identifier*; the **anchor
method** is the open call. Candidates:

| Option | Fit | Trade-off |
|---|---|---|
| **`did:key` + an EVM registry** (did:ethr-style, on an L2/permissioned chain) | Recommended. Device stays `did:key`; a smart-contract registry anchors doc + status. Ties to the estate's Ethereum work (`eth-rlp`). | Need a contract + an EVM/L2 to run; gas/latency (mitigated by L2 + off-chain cache). |
| **`did:ethr`** (the DID *is* an on-chain address) | Fully on-chain, mature tooling. | Ethereum-centric; key model differs from the Noise X25519 key. |
| **Hyperledger Indy / `did:indy`** | SSI-native, built for exactly this; strong revocation + verifiable credentials. | Heaviest infra (a permissioned ledger to operate). |
| **Sidetree / ION** (batched, Bitcoin/L1-anchored) | Scales to huge fleets, cheap per-DID. | Most complex to run. |

**Recommendation:** device identity as `did:key` (already built, offline-friendly
for IoT) + a small **EVM registry contract** as the blockchain anchor, deployable
on a permissioned or L2 chain to keep cost/latency sane. The `Resolver` interface
(below) hides the chain, so this is a backend swap, not a redesign.

## Authorization (beyond identity)
Identity says *who*; authorization says *what*. Two layers:
- **Membership** — the registry records which networks a DID belongs to.
- **Verifiable Credentials** — for finer policy (which peers, which access), issue
  W3C VCs the verifier checks. A later step; membership is enough for Phase 3.

## Implementation status
Built and tested in this repo (`internal/did`, no chain required yet):
- `FromX25519` → a W3C-conformant `did:key` for the device's Noise key
  (verified against the spec's `z6LS…` X25519 encoding).
- `NewDocument` → the DID document (X25519 keyAgreement method).
- `Matches` → the handshake-key ↔ DID binding.
- `Resolver` interface + `MemoryAnchor` (in-memory VDR stand-in) + `Authorized`
  (verify key against DID, then check the registry / revocation).
- CLI: `netso did -identity <file>` prints a device's DID + DID document.

**Next:** implement `Resolver` against a real blockchain registry (the chosen
method above); have the agent register its DID at enrolment and the hub/peer verify
via `did.Authorized` in place of raw pinned-key discovery; add an off-chain cache.

## Open decisions
1. **Anchor method / chain** — EVM registry (recommended) vs did:ethr vs Indy vs ION.
2. **Public vs permissioned / L2** — cost, latency, and who runs the chain.
3. **Verifiable credentials** — adopt W3C VCs for authorization, or keep
   membership-in-registry for now?
4. **Org identities** — `did:web` for organisations/hubs alongside `did:key` for devices?
