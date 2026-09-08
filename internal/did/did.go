// Package did gives netso devices a self-sovereign identity (SSI) built on
// W3C Decentralized Identifiers (DIDs).
//
// A device's DID is derived deterministically from its Noise/X25519 static key
// as a `did:key` (multicodec + multibase base58btc). So the identity IS the
// transport key: a peer proves control of its DID simply by completing the Noise
// handshake with the key the DID commits to — no separate signature, no CA.
//
// Resolution and revocation go through a Resolver, which in production is backed
// by a blockchain registry (a verifiable data registry, VDR) so any hub or peer
// can verify a device without a central authority. MemoryAnchor is an in-memory
// stand-in for that registry, so the model works and is testable without a chain.
package did

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
)

// x25519 multicodec (0xEC) as an unsigned varint, then the 32-byte key.
var x25519Multicodec = []byte{0xEC, 0x01}

const x25519KeyLen = 32

const b58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// multibaseKey returns the multibase (base58btc, 'z') multicodec-tagged key —
// the method-specific id of a did:key and the value of publicKeyMultibase.
func multibaseKey(pub []byte) string {
	tagged := append(append([]byte{}, x25519Multicodec...), pub...)
	return "z" + base58Encode(tagged)
}

// FromX25519 returns the did:key DID for a device's X25519 (Noise) public key.
func FromX25519(pub []byte) (string, error) {
	if len(pub) != x25519KeyLen {
		return "", fmt.Errorf("did: x25519 key must be %d bytes, got %d", x25519KeyLen, len(pub))
	}
	return "did:key:" + multibaseKey(pub), nil
}

// Matches reports whether presentedPub is the key that did commits to — i.e. the
// peer that just completed the Noise handshake with presentedPub controls did.
func Matches(did string, presentedPub []byte) bool {
	want, err := FromX25519(presentedPub)
	return err == nil && want == did
}

// VerificationMethod is a key entry in a DID document.
type VerificationMethod struct {
	ID                 string `json:"id"`
	Type               string `json:"type"`
	Controller         string `json:"controller"`
	PublicKeyMultibase string `json:"publicKeyMultibase"`
}

// Document is a minimal W3C DID document for a device.
type Document struct {
	Context            []string             `json:"@context"`
	ID                 string               `json:"id"`
	VerificationMethod []VerificationMethod `json:"verificationMethod"`
	KeyAgreement       []string             `json:"keyAgreement"`
}

// NewDocument builds the DID document for a device's X25519 key. The key is a
// keyAgreement method: the device authenticates by performing the Noise handshake
// with it.
func NewDocument(pub []byte) (Document, error) {
	id, err := FromX25519(pub)
	if err != nil {
		return Document{}, err
	}
	vmID := id + "#key-agreement-1"
	return Document{
		Context: []string{"https://www.w3.org/ns/did/v1"},
		ID:      id,
		VerificationMethod: []VerificationMethod{{
			ID:                 vmID,
			Type:               "X25519KeyAgreementKey2020",
			Controller:         id,
			PublicKeyMultibase: multibaseKey(pub),
		}},
		KeyAgreement: []string{vmID},
	}, nil
}

// Resolution is the result of resolving a DID.
type Resolution struct {
	Document Document
	Revoked  bool
}

// Resolver resolves a DID to its document and revocation status. A
// blockchain-backed registry implements this in production.
type Resolver interface {
	Resolve(did string) (Resolution, error)
}

// ErrNotFound is returned when a DID is not registered in the anchor.
var ErrNotFound = errors.New("did: not registered")

// MemoryAnchor is an in-memory verifiable data registry — a local stand-in for
// the on-chain registry, so resolution and revocation are testable without a chain.
type MemoryAnchor struct {
	mu      sync.Mutex
	docs    map[string]Document
	revoked map[string]bool
}

// NewMemoryAnchor returns an empty in-memory anchor.
func NewMemoryAnchor() *MemoryAnchor {
	return &MemoryAnchor{docs: map[string]Document{}, revoked: map[string]bool{}}
}

// Register anchors a device's DID document (as an on-chain registration would).
func (a *MemoryAnchor) Register(doc Document) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.docs[doc.ID] = doc
	delete(a.revoked, doc.ID)
}

// Revoke marks a DID revoked (as a registry status update would).
func (a *MemoryAnchor) Revoke(did string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.revoked[did] = true
}

// Resolve returns the DID document and revocation status.
func (a *MemoryAnchor) Resolve(did string) (Resolution, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	doc, ok := a.docs[did]
	if !ok {
		return Resolution{}, ErrNotFound
	}
	return Resolution{Document: doc, Revoked: a.revoked[did]}, nil
}

// Authorized resolves did and reports whether a peer presenting presentedPub is
// an authorized, non-revoked holder of it. This is the SSI replacement for a raw
// pinned key: verify the key against the DID, then check the registry.
func Authorized(r Resolver, did string, presentedPub []byte) (bool, error) {
	if !Matches(did, presentedPub) {
		return false, nil
	}
	res, err := r.Resolve(did)
	if err != nil {
		return false, err
	}
	return !res.Revoked, nil
}

func base58Encode(input []byte) string {
	zeros := 0
	for zeros < len(input) && input[zeros] == 0 {
		zeros++
	}
	x := new(big.Int).SetBytes(input)
	radix := big.NewInt(58)
	mod := new(big.Int)
	var out []byte
	for x.Sign() > 0 {
		x.DivMod(x, radix, mod)
		out = append(out, b58Alphabet[mod.Int64()])
	}
	for i := 0; i < zeros; i++ {
		out = append(out, b58Alphabet[0])
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return string(out)
}

// MethodID returns the method-specific identifier (everything after "did:key:").
func MethodID(did string) string { return strings.TrimPrefix(did, "did:key:") }
