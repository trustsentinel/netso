#!/bin/sh
# Runs in the e2e container. Asserts the Phase 1 flow: discovery, an enrolled
# client's brokered shell, network isolation, and refusal of an unknown peer.
set -u
HUB="http://hub:8443"
fail() { echo "RESULT: FAIL — $1"; exit 1; }

count_peers() { curl -fsS "$HUB/peers?network=$1" 2>/dev/null | grep -o '"name"' | wc -l | tr -d ' '; }

echo "[0] wait for both peers to register"
i=0
while [ "$(count_peers prod)" != "2" ]; do
  i=$((i + 1)); [ "$i" -gt 40 ] && fail "peers did not register"; sleep 1
done

echo "[1] discovery"
netso peers -hub "$HUB" -network prod

echo "[2] brokered shell to peerA (enrolled client, key auto-pinned from discovery)"
OUT=$(netso ssh -hub "$HUB" -network prod -peer peerA -identity /state/client.id \
      -exec 'echo NETSO:$((6*7))' 2>/dev/null || true)
echo "$OUT" | grep -q 'NETSO:42' || fail "command did not round-trip"
echo "    ok: NETSO:42 executed over the encrypted broker"

echo "[3] network isolation: 'staging' has no peers"
[ "$(count_peers staging)" = "0" ] || fail "network isolation leaked"
echo "    ok"

echo "[4] unknown peer is refused (no shell)"
BAD=$(netso ssh -hub "$HUB" -network prod -peer ghost -identity /state/client.id \
      -exec 'echo LEAK' 2>/dev/null || true)
echo "$BAD" | grep -q 'LEAK' && fail "unknown peer produced output"
echo "    ok"

echo "RESULT: PASS  (networks + discovery + enrolled, mutually-authenticated brokered shell)"
