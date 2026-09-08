package main

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// HubReconciler reconciles a Hub CR into a netso-hub Deployment + Service.
type HubReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Image  string
}

func (r *HubReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	hub := newU(hubGVK)
	if err := r.Get(ctx, req.NamespacedName, hub); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	image := nestedString(hub, "spec", "image")
	if image == "" {
		image = r.Image
	}
	replicas := int32(nestedInt(hub, 1, "spec", "replicas"))
	name, ns := hub.GetName(), hub.GetNamespace()
	labels := map[string]string{"app.kubernetes.io/name": "netso-hub", "netso.trustsentinel.eu/hub": name}

	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, dep, func() error {
		dep.Labels = labels
		dep.Spec.Replicas = &replicas
		dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: labels}
		dep.Spec.Template.ObjectMeta.Labels = labels
		dep.Spec.Template.Spec.Containers = []corev1.Container{{
			Name:           "hub",
			Image:          image,
			Command:        []string{"netso-hub", "-addr", ":8443"},
			Ports:          []corev1.ContainerPort{{Name: "ws", ContainerPort: 8443}},
			ReadinessProbe: httpProbe("/healthz", 8443),
		}}
		return controllerutil.SetControllerReference(hub, dep, r.Scheme)
	}); err != nil {
		return ctrl.Result{}, err
	}

	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Labels = labels
		svc.Spec.Selector = labels
		svc.Spec.Ports = []corev1.ServicePort{{Name: "ws", Port: 8443, TargetPort: intstr.FromInt(8443)}}
		return controllerutil.SetControllerReference(hub, svc, r.Scheme)
	}); err != nil {
		return ctrl.Result{}, err
	}

	_ = unstructured.SetNestedField(hub.Object, dep.Status.ReadyReplicas > 0, "status", "ready")
	_ = unstructured.SetNestedField(hub.Object, fmt.Sprintf("%s:8443", name), "status", "service")
	if err := r.Status().Update(ctx, hub); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// PeerReconciler reconciles a Peer CR into a netso-agent Deployment that dials
// the referenced Hub and joins the given network.
type PeerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Image  string
}

func (r *PeerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	peer := newU(peerGVK)
	if err := r.Get(ctx, req.NamespacedName, peer); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	hubRef := nestedString(peer, "spec", "hubRef")
	if hubRef == "" {
		return ctrl.Result{}, nil // schema requires it; guard anyway
	}
	network := nestedString(peer, "spec", "network")
	if network == "" {
		network = "default"
	}
	peerName := nestedString(peer, "spec", "peerName")
	if peerName == "" {
		peerName = peer.GetName()
	}
	shell := nestedString(peer, "spec", "shell")
	if shell == "" {
		shell = "/bin/sh"
	}
	image := nestedString(peer, "spec", "image")
	if image == "" {
		image = r.Image
	}
	authSecret := nestedString(peer, "spec", "authorizedClientsSecret")
	name, ns := peer.GetName(), peer.GetNamespace()
	labels := map[string]string{"app.kubernetes.io/name": "netso-agent", "netso.trustsentinel.eu/peer": name}

	container := corev1.Container{
		Name:  "agent",
		Image: image,
		Command: []string{
			"netso-agent",
			"-hub", fmt.Sprintf("http://%s:8443", hubRef),
			"-network", network,
			"-name", peerName,
			"-shell", shell,
		},
	}
	if authSecret != "" {
		container.Command = append(container.Command, "-authorized-clients", "/etc/netso/authorized/authorized_clients")
		container.VolumeMounts = []corev1.VolumeMount{{Name: "authorized", MountPath: "/etc/netso/authorized", ReadOnly: true}}
	}

	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, dep, func() error {
		one := int32(1)
		dep.Labels = labels
		dep.Spec.Replicas = &one
		dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: labels}
		dep.Spec.Template.ObjectMeta.Labels = labels
		dep.Spec.Template.Spec.Containers = []corev1.Container{container}
		if authSecret != "" {
			dep.Spec.Template.Spec.Volumes = []corev1.Volume{{
				Name:         "authorized",
				VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: authSecret}},
			}}
		}
		return controllerutil.SetControllerReference(peer, dep, r.Scheme)
	}); err != nil {
		return ctrl.Result{}, err
	}

	_ = unstructured.SetNestedField(peer.Object, dep.Status.ReadyReplicas > 0, "status", "ready")
	if err := r.Status().Update(ctx, peer); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func httpProbe(path string, port int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:  corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: path, Port: intstr.FromInt(int(port))}},
		PeriodSeconds: 5,
	}
}

func nestedString(u *unstructured.Unstructured, fields ...string) string {
	v, _, _ := unstructured.NestedString(u.Object, fields...)
	return v
}

func nestedInt(u *unstructured.Unstructured, def int64, fields ...string) int64 {
	v, found, err := unstructured.NestedInt64(u.Object, fields...)
	if err != nil || !found {
		return def
	}
	return v
}
