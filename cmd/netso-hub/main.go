// Command netso-hub is the Phase 1 control plane: peers register per network,
// clients discover them (GET /peers) and reach a peer's shell (WS /connect),
// which the hub relays to the peer's agent (WS /agent). The client and agent run
// an end-to-end Noise session on top, so the hub only ever relays ciphertext.
package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"

	"github.com/gorilla/websocket"

	"github.com/trustsentinel/netso/internal/registry"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // sessions are E2E-auth'd
}

type hub struct {
	reg *registry.Registry
}

// /agent?network=X&name=Y&pubkey=B64 — a peer registers and waits to be brokered.
func (h *hub) handleAgent(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	network, name, pubkey := q.Get("network"), q.Get("name"), q.Get("pubkey")
	if network == "" || name == "" {
		http.Error(w, "network and name are required", http.StatusBadRequest)
		return
	}
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	peer := registry.Peer{Network: network, Name: name, PubKey: pubkey}
	if prev := h.reg.Add(peer, c); prev != nil {
		if pc, ok := prev.(*websocket.Conn); ok {
			pc.Close()
		}
	}
	log.Printf("agent registered: network=%s name=%s", network, name)
	// The connection sits idle in the registry until a client is brokered onto it.
	// (Phase 1: idle agents are not read; stale entries clear on a failed relay.)
}

// GET /peers?network=X — discovery: who is on this network.
func (h *hub) handlePeers(w http.ResponseWriter, r *http.Request) {
	network := r.URL.Query().Get("network")
	if network == "" {
		http.Error(w, "network is required", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(h.reg.List(network))
}

// /connect?network=X&peer=Y — a client reaches peer Y; the hub relays.
func (h *hub) handleConnect(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	network, peerName := q.Get("network"), q.Get("peer")
	if network == "" || peerName == "" {
		http.Error(w, "network and peer are required", http.StatusBadRequest)
		return
	}
	client, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	_, agentConn, ok := h.reg.Take(network, peerName)
	if !ok {
		log.Printf("connect: no peer %q on network %q", peerName, network)
		client.Close()
		return
	}
	agent := agentConn.(*websocket.Conn)
	log.Printf("brokering client<->%s/%s (relaying ciphertext)", network, peerName)
	relay(client, agent)
	log.Printf("session closed: %s/%s", network, peerName)
}

func relay(a, b *websocket.Conn) {
	done := make(chan struct{}, 2)
	go copyMsgs(a, b, done)
	go copyMsgs(b, a, done)
	<-done
	a.Close()
	b.Close()
	<-done
}

func copyMsgs(dst, src *websocket.Conn, done chan struct{}) {
	for {
		t, msg, err := src.ReadMessage()
		if err != nil {
			break
		}
		if err := dst.WriteMessage(t, msg); err != nil {
			break
		}
	}
	done <- struct{}{}
}

func main() {
	addr := flag.String("addr", ":8443", "listen address")
	flag.Parse()

	h := &hub{reg: registry.New()}
	mux := http.NewServeMux()
	mux.HandleFunc("/agent", h.handleAgent)
	mux.HandleFunc("/connect", h.handleConnect)
	mux.HandleFunc("/peers", h.handlePeers)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	log.Printf("netso-hub listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
