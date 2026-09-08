//go:build js

// Command netso-wasm is the browser client, compiled to WebAssembly. It runs the
// same Noise IK session as the netso CLI (internal/secure) over a browser
// WebSocket (internal/transport JSConn) and exposes a small JS API the page wires
// to xterm.js — so a brokered, end-to-end-encrypted shell to a peer works in the
// browser, not only from the CLI.
//
// Build: GOOS=js GOARCH=wasm go build -o web/netso.wasm ./cmd/netso-wasm
package main

import (
	"syscall/js"

	"github.com/trustsentinel/netso/internal/secure"
	"github.com/trustsentinel/netso/internal/transport"
)

func main() {
	js.Global().Set("netsoConnect", js.FuncOf(netsoConnect))
	select {} // keep the runtime alive for callbacks
}

// netsoConnect(config) -> { send(str), close() }
// config: { wsURL, agentPub, onData(Uint8Array), onStatus(str), onClose() }
// The page builds wsURL (e.g. ws://hub/connect?network=X&peer=Y) and supplies the
// peer's public key (from discovery) to pin.
func netsoConnect(this js.Value, args []js.Value) any {
	cfg := args[0]
	wsURL := cfg.Get("wsURL").String()
	agentPub := cfg.Get("agentPub").String()
	onData := cfg.Get("onData")
	onStatus := cfg.Get("onStatus")
	onClose := cfg.Get("onClose")

	status := func(s string) {
		if onStatus.Truthy() {
			onStatus.Invoke(s)
		}
	}
	closed := func() {
		if onClose.Truthy() {
			onClose.Invoke()
		}
	}

	sendCh := make(chan []byte, 32)
	var sess *secure.Session

	go func() {
		if agentPub == "" {
			status("error: peer key required")
			closed()
			return
		}
		pin, derr := secure.DecodePublic(agentPub)
		if derr != nil {
			status("error: bad peer key")
			closed()
			return
		}

		status("connecting")
		conn, err := transport.DialWS(wsURL)
		if err != nil {
			status("error: " + err.Error())
			closed()
			return
		}
		kp, err := secure.GenerateKeypair() // ephemeral browser identity for the demo
		if err != nil {
			status("keygen error: " + err.Error())
			conn.Close()
			closed()
			return
		}

		status("handshaking")
		sess, err = secure.Handshake(conn, secure.Config{Static: kp, Initiator: true, PeerStatic: pin})
		if err != nil {
			status("auth failed: " + err.Error())
			conn.Close()
			closed()
			return
		}
		status("connected: " + secure.EncodePublic(sess.PeerStatic))

		go func() {
			for {
				data, rerr := sess.Read()
				if len(data) > 0 && onData.Truthy() {
					onData.Invoke(bytesToUint8(data))
				}
				if rerr != nil {
					break
				}
			}
			status("closed")
			closed()
		}()
		for b := range sendCh {
			if werr := sess.Write(b); werr != nil {
				break
			}
		}
	}()

	obj := js.Global().Get("Object").New()
	obj.Set("send", js.FuncOf(func(this js.Value, a []js.Value) any {
		if len(a) > 0 {
			select {
			case sendCh <- []byte(a[0].String()):
			default:
			}
		}
		return nil
	}))
	obj.Set("close", js.FuncOf(func(this js.Value, a []js.Value) any {
		if sess != nil {
			sess.Close()
		}
		return nil
	}))
	return obj
}

func bytesToUint8(b []byte) js.Value {
	u8 := js.Global().Get("Uint8Array").New(len(b))
	js.CopyBytesToJS(u8, b)
	return u8
}
