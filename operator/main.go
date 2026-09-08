// Command netso-operator is a Kubernetes controller for the netso Custom
// Resources (Hub, Peer). Apply a Hub/Peer object and the operator reconciles it
// into the underlying Deployments/Services — netso "offered in k8s" declaratively.
//
// Uses unstructured access for the CRs (no code generation), and typed clients
// for the workloads it manages.
package main

import (
	"flag"
	"os"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	group   = "netso.trustsentinel.eu"
	version = "v1alpha1"
)

var (
	hubGVK  = schema.GroupVersionKind{Group: group, Version: version, Kind: "Hub"}
	peerGVK = schema.GroupVersionKind{Group: group, Version: version, Kind: "Peer"}
)

func newU(gvk schema.GroupVersionKind) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	return u
}

func main() {
	image := flag.String("netso-image", envOr("NETSO_IMAGE", "netso:local"), "netso container image the operator deploys")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		fatal("scheme", err)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{Scheme: scheme})
	if err != nil {
		fatal("manager", err)
	}

	if err := ctrl.NewControllerManagedBy(mgr).
		For(newU(hubGVK)).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(&HubReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme(), Image: *image}); err != nil {
		fatal("hub controller", err)
	}

	if err := ctrl.NewControllerManagedBy(mgr).
		For(newU(peerGVK)).
		Owns(&appsv1.Deployment{}).
		Complete(&PeerReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme(), Image: *image}); err != nil {
		fatal("peer controller", err)
	}

	ctrl.Log.Info("starting netso-operator", "netso-image", *image)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		fatal("manager exited", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func fatal(msg string, err error) {
	ctrl.Log.Error(err, msg)
	os.Exit(1)
}
