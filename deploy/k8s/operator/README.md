# netso Kubernetes operator

Run netso in Kubernetes **declaratively, via Custom Resources**. You `kubectl
apply` a `Hub`, a `Network`, and `Peer` objects; the operator reconciles them into
the underlying Deployments and Services.

API group: **`netso.trustsentinel.eu/v1alpha1`**

| Kind | You declare | Operator creates |
|---|---|---|
| **Hub** (`nhub`) | a control-plane instance (image, replicas) | a `netso-hub` **Deployment** + a **Service** named after the Hub |
| **Peer** (`npeer`) | a device on a network (`hubRef`, `network`, name, optional enrollment secret) | a `netso-agent` **Deployment** that dials the Hub and joins the network |
| **Network** (`nnet`) | a logical network (description) | declarative for now (peer-count status is future work) |

Deletion is garbage-collected: the workloads carry an owner reference to their
custom object, so `kubectl delete peer peer-a` removes its agent Deployment.

## Install
```bash
kubectl apply -f deploy/k8s/operator/crds.yaml       # the custom objects' API
kubectl apply -f deploy/k8s/operator/operator.yaml   # namespace + RBAC + controller
```
The operator deploys the image passed as `-netso-image` (default `netso:local`);
set it in `operator.yaml` to your registry image. The cluster must be able to pull
both the operator image and that netso image.

## Use it
```bash
kubectl apply -f deploy/k8s/operator/samples/netso.yaml
kubectl get hubs,networks,peers
#   NAME   READY   SERVICE
#   main   true    main:8443
#   NAME   NETWORK   HUB    READY
#   peer-a prod      main   true
#   peer-b prod      main   true

# reach a peer (port-forward the hub, then use the netso CLI against it)
kubectl port-forward svc/main 8443:8443 &
netso ssh -hub http://localhost:8443 -network prod -peer peer-a -identity ~/.netso/id
```

## One-command test (throwaway kind cluster)
```bash
deploy/k8s/operator/kind-test.sh
```
Builds the netso + operator images, creates a kind cluster, installs the CRDs +
operator, applies the sample custom objects, and asserts the operator reconciled
them into a hub + two agents and that both peers registered. Expected tail:
```
K8S OPERATOR RESULT: PASS
```

## Design notes & limits (MVP)
- Two reconcilers (Hub, Peer) built on **controller-runtime**, using *unstructured*
  access for the CRs — no code generation, so the whole operator is a small,
  self-contained module (`operator/`), separate from the lean core netso module.
- **Ephemeral peer identity** by default (its key/DID is fresh per pod); mount a
  stable one with `spec.authorizedClientsSecret` + (future) an identity secret.
- **No ingress/TLS yet** — the Hub Service is ClusterIP; put an Ingress +
  cert-manager in front for browser access from outside the cluster.
- **Roadmap:** Network peer-count status; Hub ingress/TLS + `-webdir` (browser
  client); Peer identity Secret for a stable DID (ties to `docs/identity.md`);
  a typed API + Helm chart; agent as a DaemonSet option.
