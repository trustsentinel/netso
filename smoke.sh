#!/bin/sh
# Local (no-Docker) smoke test: hub + two agents on a network, then discover the
# peers and open a brokered, mutually-authenticated shell to one of them.
set -e
cd "$(dirname "$0")"
BIN=$(mktemp -d); KEYS=$(mktemp -d); PORT=18443
HUB="http://localhost:$PORT"

cleanup() {
  for p in "$HUB_PID" "$A_PID" "$B_PID"; do [ -n "$p" ] && kill "$p" 2>/dev/null || true; done
  rm -rf "$BIN" "$KEYS"
}
trap cleanup EXIT

echo "building..."
go build -o "$BIN/hub"   ./cmd/netso-hub
go build -o "$BIN/agent" ./cmd/netso-agent
go build -o "$BIN/netso" ./cmd/netso

echo "starting hub..."
"$BIN/hub" -addr ":$PORT" >"$BIN/hub.log" 2>&1 &
HUB_PID=$!
i=0; until curl -fsS "$HUB/healthz" >/dev/null 2>&1; do i=$((i+1)); [ "$i" -gt 50 ] && { cat "$BIN/hub.log"; exit 1; }; sleep 0.1; done

echo "starting agents peerA, peerB on network 'prod'..."
"$BIN/agent" -hub "$HUB" -network prod -name peerA >"$BIN/a.log" 2>&1 & A_PID=$!
"$BIN/agent" -hub "$HUB" -network prod -name peerB >"$BIN/b.log" 2>&1 & B_PID=$!

# wait until both peers are discoverable
i=0
while [ "$(curl -fsS "$HUB/peers?network=prod" 2>/dev/null | grep -o '"name"' | wc -l | tr -d ' ')" != "2" ]; do
  i=$((i+1)); [ "$i" -gt 50 ] && { echo "peers did not register"; cat "$BIN/a.log" "$BIN/b.log"; exit 1; }; sleep 0.1
done

echo "== discovery =="
"$BIN/netso" peers -hub "$HUB" -network prod

echo "== brokered shell to peerA (key auto-pinned from discovery) =="
OUT=$("$BIN/netso" ssh -hub "$HUB" -network prod -peer peerA -identity "$KEYS/client.id" \
      -exec 'echo NETSO:$((6*7))' 2>"$BIN/ssh.log" || true)
echo "$OUT" | tr -d '\r' | grep -a 'NETSO:42' | head -1

if echo "$OUT" | grep -q 'NETSO:42'; then
  echo "SMOKE: PASS (discovery + brokered mutually-authenticated shell)"
else
  echo "SMOKE: FAIL"; echo "--- ssh.log ---"; cat "$BIN/ssh.log"; echo "--- a.log ---"; cat "$BIN/a.log"; exit 1
fi
