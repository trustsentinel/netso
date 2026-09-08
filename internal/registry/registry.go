// Package registry tracks the peers currently connected to a hub, grouped by
// network. It backs discovery ("who is on network X?") and brokering ("give me
// the live connection for peer Y on network X"). In-memory for Phase 1; a
// federated hub would replicate this.
package registry

import (
	"sort"
	"sync"
)

// Peer is the discovery metadata advertised for a connected peer.
type Peer struct {
	Network string `json:"network"`
	Name    string `json:"name"`
	PubKey  string `json:"pubkey"` // base64 Noise static public key
}

type record struct {
	peer Peer
	conn any // the live hub-side connection (e.g. *websocket.Conn)
}

// Registry is a concurrency-safe network → name → peer store.
type Registry struct {
	mu sync.Mutex
	m  map[string]map[string]record
}

func New() *Registry {
	return &Registry{m: make(map[string]map[string]record)}
}

// Add registers (or replaces) a peer's live connection, returning the previous
// connection for that (network, name) if one existed — so the hub can close a
// stale registration.
func (r *Registry) Add(p Peer, conn any) any {
	r.mu.Lock()
	defer r.mu.Unlock()
	net, ok := r.m[p.Network]
	if !ok {
		net = make(map[string]record)
		r.m[p.Network] = net
	}
	prev := net[p.Name].conn
	net[p.Name] = record{peer: p, conn: conn}
	return prev
}

// Remove deregisters a peer, but only if its current connection matches conn
// (so a newer registration for the same name is not clobbered).
func (r *Registry) Remove(network, name string, conn any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	net, ok := r.m[network]
	if !ok {
		return
	}
	if cur, ok := net[name]; ok && cur.conn == conn {
		delete(net, name)
		if len(net) == 0 {
			delete(r.m, network)
		}
	}
}

// Take atomically removes and returns a peer's connection for brokering, so a
// peer serves one session at a time (it re-registers when the session ends).
func (r *Registry) Take(network, name string) (Peer, any, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	net, ok := r.m[network]
	if !ok {
		return Peer{}, nil, false
	}
	rec, ok := net[name]
	if !ok {
		return Peer{}, nil, false
	}
	delete(net, name)
	if len(net) == 0 {
		delete(r.m, network)
	}
	return rec.peer, rec.conn, true
}

// Networks returns each network and how many peers are currently on it (for
// monitoring/status).
func (r *Registry) Networks() map[string]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	counts := make(map[string]int, len(r.m))
	for network, peers := range r.m {
		counts[network] = len(peers)
	}
	return counts
}

// List returns the peers on a network, sorted by name (for discovery).
func (r *Registry) List(network string) []Peer {
	r.mu.Lock()
	defer r.mu.Unlock()
	peers := make([]Peer, 0, len(r.m[network]))
	for _, rec := range r.m[network] {
		peers = append(peers, rec.peer)
	}
	sort.Slice(peers, func(i, j int) bool { return peers[i].Name < peers[j].Name })
	return peers
}
