// Command netso is the client CLI: manage an identity, discover peers on a
// network, and open a brokered, end-to-end-encrypted shell to a peer.
//
//	netso keygen -identity ~/.netso/id
//	netso peers  -hub http://hub:8443 -network prod
//	netso ssh    -hub http://hub:8443 -network prod -peer web -identity ~/.netso/id
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"

	"github.com/gorilla/websocket"
	"golang.org/x/term"

	"github.com/trustsentinel/netso/internal/hubclient"
	"github.com/trustsentinel/netso/internal/secure"
	"github.com/trustsentinel/netso/internal/transport"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "keygen":
		cmdKeygen(os.Args[2:])
	case "peers":
		cmdPeers(os.Args[2:])
	case "ssh":
		cmdSSH(os.Args[2:])
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: netso <keygen|peers|ssh> [flags]")
	os.Exit(2)
}

func cmdKeygen(args []string) {
	fs := flag.NewFlagSet("keygen", flag.ExitOnError)
	identity := fs.String("identity", "", "identity file to create (or load if it exists)")
	_ = fs.Parse(args)
	if *identity == "" {
		log.Fatal("keygen: -identity is required")
	}
	kp, err := secure.LoadOrCreateIdentity(*identity)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(secure.EncodePublic(kp.Public))
}

func cmdPeers(args []string) {
	fs := flag.NewFlagSet("peers", flag.ExitOnError)
	hub := fs.String("hub", "http://localhost:8443", "hub base URL")
	network := fs.String("network", "default", "network to list")
	_ = fs.Parse(args)

	peers, err := hubclient.Peers(*hub, *network)
	if err != nil {
		log.Fatalf("discovery: %v", err)
	}
	if len(peers) == 0 {
		fmt.Printf("no peers on network %q\n", *network)
		return
	}
	fmt.Printf("%-20s %s\n", "NAME", "PUBKEY")
	for _, p := range peers {
		fmt.Printf("%-20s %s\n", p.Name, p.PubKey)
	}
}

func cmdSSH(args []string) {
	fs := flag.NewFlagSet("ssh", flag.ExitOnError)
	hub := fs.String("hub", "http://localhost:8443", "hub base URL")
	network := fs.String("network", "default", "network")
	peer := fs.String("peer", "", "peer name to connect to (required)")
	identity := fs.String("identity", "", "client identity file (created on first use)")
	priv := fs.String("key", "", "base64 static private key (with -pubkey; overridden by -identity)")
	pub := fs.String("pubkey", "", "base64 static public key (with -key)")
	authAgent := fs.String("authorized-agent", "", "pin this base64 peer key (default: from discovery)")
	execCmd := fs.String("exec", "", "run one command then exit (non-interactive)")
	_ = fs.Parse(args)

	if *peer == "" {
		log.Fatal("ssh: -peer is required")
	}
	kp, err := secure.ResolveIdentity(*identity, *priv, *pub)
	if err != nil {
		log.Fatal(err)
	}

	// Pin the peer's key: use -authorized-agent if given, else look it up via
	// discovery (hub-coordinated; hardened by SSI in a later phase).
	pinB64 := *authAgent
	if pinB64 == "" {
		peers, derr := hubclient.Peers(*hub, *network)
		if derr != nil {
			log.Fatalf("discovery: %v", derr)
		}
		for _, p := range peers {
			if p.Name == *peer {
				pinB64 = p.PubKey
				break
			}
		}
		if pinB64 == "" {
			log.Fatalf("peer %q not found on network %q (try: netso peers)", *peer, *network)
		}
	}
	pin, err := secure.DecodePublic(pinB64)
	if err != nil {
		log.Fatalf("bad peer key: %v", err)
	}

	dialURL := hubclient.WSBase(*hub) + "/connect?network=" + url.QueryEscape(*network) +
		"&peer=" + url.QueryEscape(*peer)
	c, _, err := websocket.DefaultDialer.Dial(dialURL, nil)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	conn := transport.NewWSConn(c)

	sess, err := secure.Handshake(conn, secure.Config{Static: kp, Initiator: true, PeerStatic: pin})
	if err != nil {
		log.Fatalf("handshake/auth failed: %v", err)
	}

	if *execCmd != "" {
		runExec(sess, *execCmd)
		return
	}
	runInteractive(sess)
}

func runExec(sess *secure.Session, cmd string) {
	if err := sess.Write([]byte(cmd + "; exit\n")); err != nil {
		log.Fatalf("write: %v", err)
	}
	for {
		data, rerr := sess.Read()
		if len(data) > 0 {
			os.Stdout.Write(data)
		}
		if rerr != nil {
			return
		}
	}
}

func runInteractive(sess *secure.Session) {
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		if old, err := term.MakeRaw(fd); err == nil {
			defer func() { _ = term.Restore(fd, old) }()
		}
	}
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, rerr := os.Stdin.Read(buf)
			if n > 0 {
				if werr := sess.Write(buf[:n]); werr != nil {
					break
				}
			}
			if rerr != nil {
				break
			}
		}
	}()
	for {
		data, rerr := sess.Read()
		if len(data) > 0 {
			os.Stdout.Write(data)
		}
		if rerr != nil {
			if rerr != io.EOF {
				log.Printf("session closed: %v", rerr)
			}
			return
		}
	}
}
