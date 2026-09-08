// Command netso-agent runs on a peer. It dials OUT to the hub (no inbound port),
// registers on a network under a name, and — when a client is brokered to it —
// completes a mutually-authenticated Noise handshake and serves a PTY shell over
// the encrypted session.
package main

import (
	"flag"
	"log"
	"net/url"
	"time"

	"github.com/gorilla/websocket"

	"github.com/trustsentinel/netso/internal/hubclient"
	"github.com/trustsentinel/netso/internal/secure"
	"github.com/trustsentinel/netso/internal/shell"
	"github.com/trustsentinel/netso/internal/transport"
)

func main() {
	hub := flag.String("hub", "http://localhost:8443", "hub base URL")
	network := flag.String("network", "default", "network to join")
	name := flag.String("name", "", "peer name (required, unique within the network)")
	shellPath := flag.String("shell", "/bin/sh", "shell to broker")
	identity := flag.String("identity", "", "path to a persistent device identity (created on first use)")
	priv := flag.String("key", "", "base64 static private key (with -pubkey; overridden by -identity)")
	pub := flag.String("pubkey", "", "base64 static public key (with -key)")
	authClient := flag.String("authorized-client", "", "base64 client public key allowed to connect")
	authClients := flag.String("authorized-clients", "", "path to an authorized-clients file (re-read each session)")
	once := flag.Bool("once", false, "serve a single session then exit")
	flag.Parse()

	if *name == "" {
		log.Fatal("-name is required")
	}
	kp, err := secure.ResolveIdentity(*identity, *priv, *pub)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("netso-agent %q on network %q, pubkey: %s", *name, *network, secure.EncodePublic(kp.Public))

	var staticAllowed [][]byte
	if *authClient != "" {
		pk, derr := secure.DecodePublic(*authClient)
		if derr != nil {
			log.Fatalf("bad -authorized-client: %v", derr)
		}
		staticAllowed = append(staticAllowed, pk)
	}
	if *authClients == "" && len(staticAllowed) == 0 {
		log.Print("WARNING: no -authorized-client(s) set; accepting any authenticated client (demo only)")
	}
	allowlist := func() [][]byte {
		allowed := append([][]byte(nil), staticAllowed...)
		if *authClients != "" {
			if fileKeys, ferr := secure.LoadAuthorizedKeys(*authClients); ferr != nil {
				log.Printf("WARNING: reading %s: %v", *authClients, ferr)
			} else {
				allowed = append(allowed, fileKeys...)
			}
		}
		return allowed
	}

	dialURL := hubclient.WSBase(*hub) + "/agent?network=" + url.QueryEscape(*network) +
		"&name=" + url.QueryEscape(*name) +
		"&pubkey=" + url.QueryEscape(secure.EncodePublic(kp.Public))

	for {
		if err := serve(dialURL, kp, allowlist, *shellPath); err != nil {
			log.Printf("session ended: %v", err)
		}
		if *once {
			return
		}
		time.Sleep(time.Second)
	}
}

func serve(dialURL string, kp secure.Keypair, allowlist func() [][]byte, shellPath string) error {
	c, _, err := websocket.DefaultDialer.Dial(dialURL, nil)
	if err != nil {
		return err
	}
	conn := transport.NewWSConn(c)
	log.Print("connected to hub, waiting to be brokered")

	sess, err := secure.Handshake(conn, secure.Config{Static: kp, Initiator: false, Authorized: allowlist()})
	if err != nil {
		conn.Close()
		return err
	}
	log.Printf("secure session established with client %s", secure.EncodePublic(sess.PeerStatic))

	sh, err := shell.Start(shellPath)
	if err != nil {
		sess.Close()
		return err
	}
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, rerr := sh.Read(buf)
			if n > 0 {
				if werr := sess.Write(buf[:n]); werr != nil {
					break
				}
			}
			if rerr != nil {
				break
			}
		}
		sess.Close()
	}()
	for {
		data, rerr := sess.Read()
		if len(data) > 0 {
			if _, werr := sh.Write(data); werr != nil {
				break
			}
		}
		if rerr != nil {
			break
		}
	}
	sh.Close()
	sess.Close()
	return nil
}
