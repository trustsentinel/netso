// Package hubclient holds the small client-side helpers for talking to a hub:
// normalizing the hub URL to ws/http and querying discovery.
package hubclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/trustsentinel/netso/internal/registry"
)

// WSBase normalizes a hub address (http(s)://, ws(s)://, or bare host:port) to a
// websocket base URL with no trailing slash.
func WSBase(hub string) string {
	switch {
	case strings.HasPrefix(hub, "wss://"), strings.HasPrefix(hub, "ws://"):
		return strings.TrimRight(hub, "/")
	case strings.HasPrefix(hub, "https://"):
		return "wss://" + strings.TrimRight(strings.TrimPrefix(hub, "https://"), "/")
	case strings.HasPrefix(hub, "http://"):
		return "ws://" + strings.TrimRight(strings.TrimPrefix(hub, "http://"), "/")
	default:
		return "ws://" + strings.TrimRight(hub, "/")
	}
}

// HTTPBase normalizes a hub address to an http(s) base URL with no trailing slash.
func HTTPBase(hub string) string {
	switch {
	case strings.HasPrefix(hub, "https://"), strings.HasPrefix(hub, "http://"):
		return strings.TrimRight(hub, "/")
	case strings.HasPrefix(hub, "wss://"):
		return "https://" + strings.TrimRight(strings.TrimPrefix(hub, "wss://"), "/")
	case strings.HasPrefix(hub, "ws://"):
		return "http://" + strings.TrimRight(strings.TrimPrefix(hub, "ws://"), "/")
	default:
		return "http://" + strings.TrimRight(hub, "/")
	}
}

// Peers queries the hub's discovery endpoint for the peers on a network.
func Peers(hub, network string) ([]registry.Peer, error) {
	u := HTTPBase(hub) + "/peers?network=" + url.QueryEscape(network)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hub /peers: %s", resp.Status)
	}
	var peers []registry.Peer
	if err := json.NewDecoder(resp.Body).Decode(&peers); err != nil {
		return nil, err
	}
	return peers, nil
}
