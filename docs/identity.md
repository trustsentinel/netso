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

## DID method — the decision: Hyperledger Indy (`did:indy`)

**Chosen anchor: Hyperledger Indy, using `did:indy`.** Indy is SSI-native — DIDs,
**AnonCreds verifiable credentials**, and **revocation registries** are all
first-class on the ledger — which is the strongest fit for netso's SSI purpose,
and its **permissioned-ledger** model aligns with netso's "hubs you operate"
posture (you run the node pool; no public chain, no gas).

Options considered:

| Option | Fit | Trade-off |
|---|---|---|
| **Hyperledger Indy / `did:indy`** ✅ chosen | SSI-native: DIDs + AnonCreds VCs + revocation registries built in; permissioned, operator-run. | Heaviest infra — a Indy node pool to run; Indy signing keys are Ed25519 (see below). |
| `did:key` + an EVM registry (did:ethr-style) | Lightest anchor; ties to the estate's Ethereum work (`eth-rlp`). | A contract + EVM/L2 to run; weaker native VC/revocation story. |
| `did:ethr` | Fully on-chain, mature tooling. | Ethereum-centric; key model differs from the Noise key. |
| Sidetree / ION | Scales cheaply to huge fleets. | Most complex to run. |

### How Indy relates to the `did:key` core already built
- **`did:key` stays** as the device's *self-certifying, offline* identifier — no
  ledger needed to simply *have* an identity (crucial for IoT).
- At **enrolment**, an endorser anchors the device on the Indy pool as a
  **`did:indy`** (a NYM transaction), with the device's keys in the DID document —
  giving decentralized resolution, revocation registries, and AnonCreds
  authorization credentials.
- **Key model:** Indy ledger transactions are **Ed25519**-signed, while netso's
  transport key is **X25519** (key agreement). So a `did:indy` device carries an
  **Ed25519** verification key (authentication / ledger) *and* the **X25519** Noise
  key as `keyAgreement`. The device authenticates the transport with the X25519 key
  (the Noise handshake, as today); the Ed25519 key is its ledger identity.
- The `Resolver` interface below is the seam: an Indy-backed resolver implements
  it later; `MemoryAnchor` + `did:key` stand in until then.

> Status: **decision made, implementation deferred** (design-only for now). No
> Indy pool is stood up yet; the tested in-memory core is what runs today.

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

**Next (deferred — design-only for now):** stand up a Hyperledger **Indy** node
pool; implement `Resolver` against it (`did:indy`); have an endorser anchor a
device's DID at enrolment; have the hub/peer verify via `did.Authorized` in place
of raw pinned-key discovery; add an off-chain resolver cache for IoT/offline.

## Open decisions
1. ~~Anchor method~~ — **decided: Hyperledger Indy / `did:indy`** (above).
2. **Indy pool topology** — how many nodes, who are the stewards/endorsers, and
   how it maps onto netso's federated-hub model.
3. **Device key handling** — issue a per-device **Ed25519** ledger key alongside
   the X25519 Noise key, and where enrolment signing happens (device vs endorser).
4. **Authorization** — model access with **AnonCreds verifiable credentials**, or
   start with plain network-membership on the ledger?
5. **Org identities** — `did:web` for organisations/hubs alongside device DIDs?
