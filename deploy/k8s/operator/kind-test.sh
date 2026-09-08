#!/bin/sh
# End-to-end test of the netso operator on a throwaway kind cluster:
# build the netso + operator images -> load -> install CRDs + operator ->
# apply Hub/Network/Peer custom objects -> verify the operator reconciled them
# into a hub Service and agent Deployments, and the peers registered.
set -e
CLUSTER="${NETSO_KIND_CLUSTER:-netso-operator-test}"
REPO_ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)"
cd "$REPO_ROOT"
DIR=deploy/k8s/operator

cleanup() { kind delete cluster --name "$CLUSTER" >/dev/null 2>&1 || true; }
trap cleanup EXIT

echo "[1/7] build images"
docker build -f deploy/compose/Dockerfile -t netso:local . >/dev/null
docker build -f operator/Dockerfile -t netso-operator:local operator >/dev/null

echo "[2/7] create kind cluster"
kind create cluster --name "$CLUSTER" >/dev/null

echo "[3/7] load images"
kind load docker-image netso:local netso-operator:local --name "$CLUSTER" >/dev/null

echo "[4/7] install CRDs + operator"
kubectl apply -f "$DIR/crds.yaml" >/dev/null
kubectl apply -f "$DIR/operator.yaml" >/dev/null
kubectl -n netso-system rollout status deploy/netso-operator --timeout=120s

echo "[5/7] apply the custom objects"
kubectl apply -f "$DIR/samples/netso.yaml" >/dev/null

echo "[6/7] wait for the operator to reconcile them into workloads"
kubectl wait --for=create deploy/main deploy/peer-a deploy/peer-b --timeout=60s >/dev/null 2>&1 || sleep 5
kubectl rollout status deploy/main   --timeout=120s
kubectl rollout status deploy/peer-a --timeout=120s
kubectl rollout status deploy/peer-b --timeout=120s

echo "[7/7] verify discovery: both peers registered with the hub"
ok=""
for i in $(seq 1 30); do
  n=$(kubectl run probe-$i --image=netso:local --restart=Never --rm -i --quiet --command -- \
        sh -c 'curl -fsS "http://main:8443/peers?network=prod" 2>/dev/null' 2>/dev/null \
        | grep -o '"name"' | wc -l | tr -d ' ')
  if [ "$n" = "2" ]; then ok=1; break; fi
  sleep 2
done

echo "----- custom objects -----"
kubectl get hubs,networks,peers
echo "----- workloads created by the operator -----"
kubectl get deploy,svc -l app.kubernetes.io/name=netso-hub
kubectl get deploy -l app.kubernetes.io/name=netso-agent
if [ -n "$ok" ]; then
  echo "K8S OPERATOR RESULT: PASS  (custom objects -> hub + 2 agents -> both peers registered)"
else
  echo "K8S OPERATOR RESULT: FAIL"; kubectl -n netso-system logs deploy/netso-operator --tail=30 || true; exit 1
fi
