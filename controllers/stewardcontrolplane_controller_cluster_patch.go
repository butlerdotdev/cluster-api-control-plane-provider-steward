// Copyright 2025 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/butlerdotdev/cluster-api-control-plane-provider-steward/api/v1alpha1"
)

const (
	defaultIngressPort = 443
	defaultGatewayPort = 6443
)

func (r *StewardControlPlaneReconciler) controlPlaneEndpoint(controlPlane *v1alpha1.StewardControlPlane, statusEndpoint string) (string, int64, error) {
	endpoint, strPort, err := net.SplitHostPort(statusEndpoint)
	if err != nil {
		return "", 0, errors.Wrap(err, "cannot split the Steward endpoint host port pair")
	}

	port, pErr := strconv.ParseInt(strPort, 10, 16)
	if pErr != nil {
		return "", 0, errors.Wrap(pErr, "cannot convert port to integer")
	}

	if ingress := controlPlane.Spec.Network.Ingress; ingress != nil {
		endpoint, port, err = parseHostnameWithDefault(ingress.Hostname, defaultIngressPort, "Ingress")
		if err != nil {
			return "", 0, err
		}
	}

	if gateway := controlPlane.Spec.Network.Gateway; gateway != nil {
		endpoint, port, err = parseHostnameWithDefault(gateway.Hostname, defaultGatewayPort, "Gateway")
		if err != nil {
			return "", 0, err
		}
	}

	return endpoint, port, nil
}

// parseHostnameWithDefault splits a hostname into host and port, applying a
// default port when none is specified.
func parseHostnameWithDefault(hostname string, defaultPort int, label string) (string, int64, error) {
	if len(strings.Split(hostname, ":")) == 1 {
		hostname += ":" + strconv.Itoa(defaultPort)
	}

	host, strPort, err := net.SplitHostPort(hostname)
	if err != nil {
		return "", 0, errors.Wrapf(err, "cannot split the Steward %s hostname host port pair", label)
	}

	port, err := strconv.ParseInt(strPort, 10, 64)
	if err != nil {
		return "", 0, errors.Wrapf(err, "cannot convert Steward %s hostname port pair", label)
	}

	return host, port, nil
}

func (r *StewardControlPlaneReconciler) patchControlPlaneEndpoint(ctx context.Context, controlPlane *v1alpha1.StewardControlPlane, hostPort string) error {
	endpoint, port, err := r.controlPlaneEndpoint(controlPlane, hostPort)
	if err != nil {
		return errors.Wrap(err, "cannot retrieve ControlPlaneEndpoint")
	}

	// Use merge patch to only update the ControlPlaneEndpoint without overwriting other spec fields
	patch := client.MergeFrom(controlPlane.DeepCopy())
	controlPlane.Spec.ControlPlaneEndpoint = capiv1beta1.APIEndpoint{
		Host: endpoint,
		Port: int32(port), //nolint:gosec
	}

	if err = r.client.Patch(ctx, controlPlane, patch); err != nil {
		return errors.Wrap(err, "cannot patch StewardControlPlane with ControlPlaneEndpoint")
	}

	return nil
}

//nolint:cyclop
func (r *StewardControlPlaneReconciler) patchCluster(ctx context.Context, cluster capiv1beta1.Cluster, controlPlane *v1alpha1.StewardControlPlane, hostPort string) error {
	if cluster.Spec.InfrastructureRef == nil {
		return errors.New("capiv1beta1.Cluster has no InfrastructureRef")
	}

	endpoint, port, err := r.controlPlaneEndpoint(controlPlane, hostPort)
	if err != nil {
		return errors.Wrap(err, "cannot retrieve ControlPlaneEndpoint")
	}

	switch cluster.Spec.InfrastructureRef.Kind {
	case "AWSCluster":
		return r.patchGenericCluster(ctx, cluster, endpoint, port, false)
	case "AzureCluster":
		return r.patchGenericCluster(ctx, cluster, endpoint, port, false)
	case "HetznerCluster":
		return r.patchGenericCluster(ctx, cluster, endpoint, port, false)
	case "IonosCloudCluster":
		return r.patchGenericCluster(ctx, cluster, endpoint, port, false)
	case "KubevirtCluster":
		return r.patchGenericCluster(ctx, cluster, endpoint, port, true)
	case "Metal3Cluster":
		return r.checkGenericCluster(ctx, cluster, endpoint, port)
	case "NutanixCluster":
		return r.patchGenericCluster(ctx, cluster, endpoint, port, true)
	case "OpenStackCluster":
		return r.patchOpenStackCluster(ctx, cluster, endpoint, port)
	case "PacketCluster":
		return r.patchGenericCluster(ctx, cluster, endpoint, port, true)
	case "ProxmoxCluster":
		return r.checkOrPatchGenericCluster(ctx, cluster, endpoint, port)
	case "TinkerbellCluster":
		return r.checkOrPatchGenericCluster(ctx, cluster, endpoint, port)
	case "VSphereCluster":
		return r.checkOrPatchGenericCluster(ctx, cluster, endpoint, port)
	default:
		if r.DynamicInfrastructureClusters.Has(cluster.Spec.InfrastructureRef.Kind) {
			return r.patchGenericCluster(ctx, cluster, endpoint, port, false)
		}

		return errors.New("unsupported infrastructure provider")
	}
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=proxmoxclusters;vsphereclusters;tinkerbellclusters,verbs=get;list;watch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=proxmoxclusters;vsphereclusters;tinkerbellclusters,verbs=patch

func (r *StewardControlPlaneReconciler) checkOrPatchGenericCluster(ctx context.Context, cluster capiv1beta1.Cluster, endpoint string, port int64) error {
	if err := r.checkGenericCluster(ctx, cluster, endpoint, port); err != nil {
		if errors.As(err, &UnmanagedControlPlaneAddressError{}) {
			return r.patchGenericCluster(ctx, cluster, endpoint, port, false)
		}

		return err
	}

	return nil
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsclusters;azureclusters;hetznerclusters;kubevirtclusters;nutanixclusters;packetclusters;ionoscloudclusters,verbs=patch;get;list;watch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=kubevirtclusters/status;nutanixclusters/status;packetclusters/status,verbs=patch

func (r *StewardControlPlaneReconciler) patchGenericCluster(ctx context.Context, cluster capiv1beta1.Cluster, endpoint string, port int64, patchStatus bool) error {
	infraCluster := unstructured.Unstructured{}

	infraCluster.SetGroupVersionKind(cluster.Spec.InfrastructureRef.GroupVersionKind())
	infraCluster.SetName(cluster.Spec.InfrastructureRef.Name)
	infraCluster.SetNamespace(cluster.Spec.InfrastructureRef.Namespace)

	if err := r.client.Get(ctx, types.NamespacedName{Name: infraCluster.GetName(), Namespace: infraCluster.GetNamespace()}, &infraCluster); err != nil {
		return errors.Wrap(err, fmt.Sprintf("cannot retrieve the %s resource", infraCluster.GetKind()))
	}

	patchHelper, err := patch.NewHelper(&infraCluster, r.client)
	if err != nil {
		return errors.Wrap(err, "unable to create patch helper")
	}

	if err = unstructured.SetNestedMap(infraCluster.Object, map[string]interface{}{
		"host": endpoint,
		"port": port,
	}, "spec", "controlPlaneEndpoint"); err != nil {
		return errors.Wrap(err, fmt.Sprintf("unable to set unstructured %s spec patch", infraCluster.GetKind()))
	}

	if patchStatus {
		if err = unstructured.SetNestedField(infraCluster.Object, true, "status", "ready"); err != nil {
			return errors.Wrap(err, fmt.Sprintf("unable to set unstructured %s status patch", infraCluster.GetKind()))
		}
	}

	if err = patchHelper.Patch(ctx, &infraCluster); err != nil {
		return errors.Wrap(err, fmt.Sprintf("cannot perform PATCH update for the %s resource", infraCluster.GetKind()))
	}

	return nil
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3clusters,verbs=get;list;watch

func (r *StewardControlPlaneReconciler) checkGenericCluster(ctx context.Context, cluster capiv1beta1.Cluster, endpoint string, port int64) error {
	gkc := unstructured.Unstructured{}

	gkc.SetGroupVersionKind(cluster.Spec.InfrastructureRef.GroupVersionKind())
	gkc.SetName(cluster.Spec.InfrastructureRef.Name)
	gkc.SetNamespace(cluster.Spec.InfrastructureRef.Namespace)

	if err := r.client.Get(ctx, types.NamespacedName{Name: gkc.GetName(), Namespace: gkc.GetNamespace()}, &gkc); err != nil {
		return errors.Wrap(err, fmt.Sprintf("cannot retrieve the %s resource", gkc.GetKind()))
	}

	cpHost, _, err := unstructured.NestedString(gkc.Object, "spec", "controlPlaneEndpoint", "host")
	if err != nil {
		return errors.Wrap(err, "cannot extract control plane endpoint host")
	}

	if cpHost == "" {
		return *NewUnmanagedControlPlaneAddressError(gkc.GetKind())
	}

	cpPort, _, err := unstructured.NestedInt64(gkc.Object, "spec", "controlPlaneEndpoint", "port")
	if err != nil {
		return errors.Wrap(err, "cannot extract control plane endpoint host")
	}

	if len(cpHost) == 0 && cpPort == 0 {
		return *NewUnmanagedControlPlaneAddressError(gkc.GetKind())
	}

	if cpHost != endpoint {
		return fmt.Errorf("the %s cluster has been provisioned with a mismatching host", gkc.GetKind())
	}

	if cpPort != port {
		return fmt.Errorf("the %s cluster has been provisioned with a mismatching port", gkc.GetKind())
	}

	return nil
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=openstackclusters,verbs=patch;get;list;watch

func (r *StewardControlPlaneReconciler) patchOpenStackCluster(ctx context.Context, cluster capiv1beta1.Cluster, endpoint string, port int64) error {
	osc := unstructured.Unstructured{}

	osc.SetGroupVersionKind(cluster.Spec.InfrastructureRef.GroupVersionKind())
	osc.SetName(cluster.Spec.InfrastructureRef.Name)
	osc.SetNamespace(cluster.Spec.InfrastructureRef.Namespace)

	if err := r.client.Get(ctx, types.NamespacedName{Name: osc.GetName(), Namespace: osc.GetNamespace()}, &osc); err != nil {
		return errors.Wrap(err, fmt.Sprintf("cannot retrieve the %s resource", osc.GetKind()))
	}

	patchHelper, err := patch.NewHelper(&osc, r.client)
	if err != nil {
		return errors.Wrap(err, "unable to create patch helper")
	}

	if err = unstructured.SetNestedField(osc.Object, endpoint, "spec", "apiServerFixedIP"); err != nil {
		return errors.Wrap(err, fmt.Sprintf("unable to set unstructured %s spec apiServerFixedIP", osc.GetKind()))
	}

	if err = unstructured.SetNestedField(osc.Object, port, "spec", "apiServerPort"); err != nil {
		return errors.Wrap(err, fmt.Sprintf("unable to set unstructured %s spec apiServerPort", osc.GetKind()))
	}

	if err = patchHelper.Patch(ctx, &osc); err != nil {
		return errors.Wrap(err, "cannot perform PATCH update for the OpenStackCluster resource")
	}

	return nil
}
