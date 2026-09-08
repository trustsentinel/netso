package did

import (
	"strings"
	"testing"
)

func key(b byte) []byte {
	k := make([]byte, x25519KeyLen)
	for i := range k {
		k[i] = b
	}
	return k
}

func TestFromX25519IsDidKeyConformant(t *testing.T) {
	d, err := FromX25519(key(0x01))
	if err != nil {
		t.Fatal(err)
	}
	// W3C did:key: an X25519 key encodes to a "z6LS..." method id.
	if !strings.HasPrefix(d, "did:key:z6LS") {
		t.Fatalf("did = %q, want did:key:z6LS…", d)
	}
	// deterministic
	d2, _ := FromX25519(key(0x01))
	if d != d2 {
		t.Fatal("DID not deterministic")
	}
	// different key → different DID
	d3, _ := FromX25519(key(0x02))
	if d == d3 {
		t.Fatal("different keys produced the same DID")
	}
}

func TestFromX25519RejectsBadLength(t *testing.T) {
	if _, err := FromX25519([]byte{1, 2, 3}); err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestMatches(t *testing.T) {
	pub := key(0x07)
	d, _ := FromX25519(pub)
	if !Matches(d, pub) {
		t.Fatal("Matches should be true for the committed key")
	}
	if Matches(d, key(0x08)) {
		t.Fatal("Matches should be false for a different key")
	}
}

func TestNewDocument(t *testing.T) {
	pub := key(0x09)
	doc, err := NewDocument(pub)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := FromX25519(pub)
	if doc.ID != id {
		t.Fatalf("doc.ID = %q, want %q", doc.ID, id)
	}
	if len(doc.VerificationMethod) != 1 || doc.VerificationMethod[0].Type != "X25519KeyAgreementKey2020" {
		t.Fatalf("unexpected verification method: %+v", doc.VerificationMethod)
	}
	if len(doc.KeyAgreement) != 1 || doc.KeyAgreement[0] != doc.VerificationMethod[0].ID {
		t.Fatalf("keyAgreement not wired to the verification method: %+v", doc)
	}
	// publicKeyMultibase is the same multibase key as the DID's method id.
	if doc.VerificationMethod[0].PublicKeyMultibase != MethodID(id) {
		t.Fatal("publicKeyMultibase should equal the did:key method id")
	}
}

func TestAuthorizedViaAnchor(t *testing.T) {
	anchor := NewMemoryAnchor()
	pub := key(0x0a)
	d, _ := FromX25519(pub)
	doc, _ := NewDocument(pub)

	// Not registered yet → not authorized (ErrNotFound surfaced).
	if ok, err := Authorized(anchor, d, pub); ok || err == nil {
		t.Fatalf("unregistered DID: ok=%v err=%v, want false + error", ok, err)
	}

	anchor.Register(doc)
	ok, err := Authorized(anchor, d, pub)
	if err != nil || !ok {
		t.Fatalf("registered DID with right key: ok=%v err=%v, want true", ok, err)
	}

	// Wrong key for this DID → not authorized, no registry lookup needed.
	if ok, _ := Authorized(anchor, d, key(0x0b)); ok {
		t.Fatal("authorized with the wrong key")
	}

	// Revoked → not authorized.
	anchor.Revoke(d)
	if ok, err := Authorized(anchor, d, pub); ok || err != nil {
		t.Fatalf("revoked DID: ok=%v err=%v, want false", ok, err)
	}
}
