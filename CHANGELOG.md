# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-09-09
Initial tagged release.

### Added
- Secure-networking platform: `netso-hub`, `netso-agent`, the `netso` CLI, and a WebAssembly browser client.
- End-to-end Noise IK encryption (hub relays ciphertext), peer registry, audit log, discovery, monitoring.
- Self-sovereign device identity (W3C `did:key`) core.
- Kubernetes operator (Hub/Network/Peer CRDs) with a kind end-to-end test, a Docker Compose demo, and CI.

[0.1.0]: https://github.com/trustsentinel/netso/releases/tag/v0.1.0
