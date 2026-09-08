#!/bin/sh
# Starts a hub (serving the browser client) + two peers for manual/browser testing.
# Prints the URL to open, then stays running.
set -e
cd "$(dirname "$0")"
PORT=18443
BIN=$(mktemp -d)

cleanup() { for p in "$HUB_PID" "$A_PID" "$B_PID"; do [ -n "$p" ] && kill "$p" 2>/dev/null || true; done; rm -rf "$BIN"; }
trap cleanup EXIT INT TERM

go build -o "$BIN/hub" ./cmd/netso-hub
go build -o "$BIN/agent" ./cmd/netso-agent
# build the browser client + copy the matching wasm runtime shim
GOROOT="$(go env GOROOT)"
if [ -f "$GOROOT/lib/wasm/wasm_exec.js" ]; then cp "$GOROOT/lib/wasm/wasm_exec.js" web/wasm_exec.js
elif [ -f "$GOROOT/misc/wasm/wasm_exec.js" ]; then cp "$GOROOT/misc/wasm/wasm_exec.js" web/wasm_exec.js; fi
GOOS=js GOARCH=wasm go build -o web/netso.wasm ./cmd/netso-wasm

"$BIN/hub" -addr ":$PORT" -webdir web >"$BIN/hub.log" 2>&1 & HUB_PID=$!
i=0; until curl -fsS "http://localhost:$PORT/healthz" >/dev/null 2>&1; do i=$((i+1)); [ "$i" -gt 50 ] && { cat "$BIN/hub.log"; exit 1; }; sleep 0.1; done

# agents accept any authenticated client (the browser uses an ephemeral key)
"$BIN/agent" -hub "http://localhost:$PORT" -network prod -name peerA >"$BIN/a.log" 2>&1 & A_PID=$!
"$BIN/agent" -hub "http://localhost:$PORT" -network prod -name peerB >"$BIN/b.log" 2>&1 & B_PID=$!
i=0; while [ "$(curl -fsS "http://localhost:$PORT/peers?network=prod" 2>/dev/null | grep -o '"name"' | wc -l | tr -d ' ')" != "2" ]; do i=$((i+1)); [ "$i" -gt 50 ] && { cat "$BIN/a.log"; exit 1; }; sleep 0.1; done

echo "READY"
echo "URL=http://localhost:$PORT/?network=prod"
while kill -0 "$HUB_PID" 2>/dev/null; do sleep 1; done
