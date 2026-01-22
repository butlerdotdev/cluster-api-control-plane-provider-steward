// Copyright 2025 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"net"
	"strings"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	scpv1alpha1 "github.com/butlerdotdev/cluster-api-control-plane-provider-steward/api/v1alpha1"
	"github.com/butlerdotdev/cluster-api-control-plane-provider-steward/pkg/externalclusterreference"
)

var ErrUnsupportedCertificateSAN = errors.New("a certificate SAN must be made of host only with no port")

//+kubebuilder:rbac:groups=steward.butlerlabs.dev,resources=tenantcontrolplanes,verbs=get;list;watch;create;update

//nolint:funlen,gocognit,cyclop,maintidx
func (r *StewardControlPlaneReconciler) createOrUpdateTenantControlPlane(ctx context.Context, remoteClient client.Client, cluster capiv1beta1.Cluster, scp scpv1alpha1.StewardControlPlane) (*stewardv1alpha1.TenantControlPlane, error) {
	tcp := &stewardv1alpha1.TenantControlPlane{}
	tcp.Name = scp.GetName()
	tcp.Namespace = scp.GetNamespace()

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		k8sClient := r.client

		var isDelegatedExternally bool

		if isDelegatedExternally = remoteClient != nil; isDelegatedExternally {
			k8sClient = remoteClient
			tcp.Name, tcp.Namespace = externalclusterreference.GenerateRemoteTenantControlPlaneNames(scp)
		}

		_, scopeErr := controllerutil.CreateOrUpdate(ctx, k8sClient, tcp, func() error {
			if tcp.Annotations == nil {
				tcp.Annotations = make(map[string]string)
			}

			for k, v := range scp.Annotations {
				if k == corev1.LastAppliedConfigAnnotation {
					continue
				}

				tcp.Annotations[k] = v
			}

			tcp.Labels = scp.Labels

			if kubeconfigSecretKey := scp.Annotations[stewardv1alpha1.KubeconfigSecretKeyAnnotation]; kubeconfigSecretKey != "" {
				tcp.Annotations[stewardv1alpha1.KubeconfigSecretKeyAnnotation] = kubeconfigSecretKey
			} else {
				delete(tcp.Annotations, stewardv1alpha1.KubeconfigSecretKeyAnnotation)
			}
			if cluster.Spec.ClusterNetwork != nil {
				// TenantControlPlane port
				if apiPort := cluster.Spec.ClusterNetwork.APIServerPort; apiPort != nil {
					tcp.Spec.NetworkProfile.Port = *apiPort
				}
				// TenantControlPlane Services CIDR
				if serviceCIDR := cluster.Spec.ClusterNetwork.Services; serviceCIDR != nil && len(serviceCIDR.CIDRBlocks) > 0 {
					tcp.Spec.NetworkProfile.ServiceCIDR = serviceCIDR.CIDRBlocks[0]
				}
				// TenantControlPlane Pods CIDR
				if podsCIDR := cluster.Spec.ClusterNetwork.Pods; podsCIDR != nil && len(podsCIDR.CIDRBlocks) > 0 {
					tcp.Spec.NetworkProfile.PodCIDR = podsCIDR.CIDRBlocks[0]
				}
				// TenantControlPlane cluster domain
				tcp.Spec.NetworkProfile.ClusterDomain = cluster.Spec.ClusterNetwork.ServiceDomain
			}
			// Replicas
			tcp.Spec.ControlPlane.Deployment.Replicas = scp.Spec.Replicas
			// Version
			// Tolerate version strings without a "v" prefix: prepend it if it's not there
			if !strings.HasPrefix(scp.Spec.Version, "v") {
				tcp.Spec.Kubernetes.Version = "v" + scp.Spec.Version
			} else {
				tcp.Spec.Kubernetes.Version = scp.Spec.Version
			}
			// Set before CoreDNS addon to allow override.
			tcp.Spec.NetworkProfile.DNSServiceIPs = scp.Spec.Network.DNSServiceIPs
			// Steward addons and CoreDNS overrides
			tcp.Spec.Addons = scp.Spec.Addons.AddonsSpec
			if scp.Spec.Addons.CoreDNS != nil {
				tcp.Spec.NetworkProfile.DNSServiceIPs = scp.Spec.Addons.CoreDNS.DNSServiceIPs

				if scp.Spec.Addons.CoreDNS.AddonSpec == nil {
					scp.Spec.Addons.CoreDNS.AddonSpec = &stewardv1alpha1.AddonSpec{}
				}

				tcp.Spec.Addons.CoreDNS = scp.Spec.Addons.CoreDNS.AddonSpec
			}
			// Steward specific options
			tcp.Spec.DataStore = scp.Spec.DataStoreName
			if scp.Spec.DataStoreSchema != "" {
				tcp.Spec.DataStoreSchema = scp.Spec.DataStoreSchema
			}
			if scp.Spec.DataStoreUsername != "" {
				tcp.Spec.DataStoreUsername = scp.Spec.DataStoreUsername
			}
			tcp.Spec.Kubernetes.AdmissionControllers = scp.Spec.AdmissionControllers
			tcp.Spec.ControlPlane.Deployment.RegistrySettings.Registry = scp.Spec.ContainerRegistry
			// Volume mounts
			if tcp.Spec.ControlPlane.Deployment.AdditionalVolumeMounts == nil {
				tcp.Spec.ControlPlane.Deployment.AdditionalVolumeMounts = &stewardv1alpha1.AdditionalVolumeMounts{}
			}

			tcp.Spec.ControlPlane.Deployment.AdditionalVolumeMounts.ControllerManager = scp.Spec.ControllerManager.ExtraVolumeMounts
			tcp.Spec.ControlPlane.Deployment.AdditionalVolumeMounts.Scheduler = scp.Spec.Scheduler.ExtraVolumeMounts
			tcp.Spec.ControlPlane.Deployment.AdditionalVolumeMounts.APIServer = scp.Spec.ApiServer.ExtraVolumeMounts
			// Extra args
			if tcp.Spec.ControlPlane.Deployment.ExtraArgs == nil {
				tcp.Spec.ControlPlane.Deployment.ExtraArgs = &stewardv1alpha1.ControlPlaneExtraArgs{}
			}

			tcp.Spec.ControlPlane.Deployment.ExtraArgs.ControllerManager = scp.Spec.ControllerManager.ExtraArgs
			tcp.Spec.ControlPlane.Deployment.ExtraArgs.Scheduler = scp.Spec.Scheduler.ExtraArgs
			tcp.Spec.ControlPlane.Deployment.ExtraArgs.APIServer = scp.Spec.ApiServer.ExtraArgs
			tcp.Spec.ControlPlane.Deployment.ExtraArgs.Kine = scp.Spec.Kine.ExtraArgs
			// Resources
			if tcp.Spec.ControlPlane.Deployment.Resources == nil {
				tcp.Spec.ControlPlane.Deployment.Resources = &stewardv1alpha1.ControlPlaneComponentsResources{}
			}

			tcp.Spec.ControlPlane.Deployment.Resources.ControllerManager = &scp.Spec.ControllerManager.Resources
			tcp.Spec.ControlPlane.Deployment.Resources.Scheduler = &scp.Spec.Scheduler.Resources
			tcp.Spec.ControlPlane.Deployment.Resources.APIServer = &scp.Spec.ApiServer.Resources
			tcp.Spec.ControlPlane.Deployment.Resources.Kine = &scp.Spec.Kine.Resources
			// Container image overrides
			tcp.Spec.ControlPlane.Deployment.RegistrySettings.ControllerManagerImage = scp.Spec.ControllerManager.ContainerImageName
			tcp.Spec.ControlPlane.Deployment.RegistrySettings.SchedulerImage = scp.Spec.Scheduler.ContainerImageName
			tcp.Spec.ControlPlane.Deployment.RegistrySettings.APIServerImage = scp.Spec.ApiServer.ContainerImageName
			// Kubelet
			tcp.Spec.Kubernetes.Kubelet = scp.Spec.Kubelet
			// Network
			tcp.Spec.NetworkProfile.Address = scp.Spec.Network.ServiceAddress
			tcp.Spec.ControlPlane.Service.ServiceType = scp.Spec.Network.ServiceType
			tcp.Spec.ControlPlane.Service.AdditionalMetadata.Labels = scp.Spec.Network.ServiceLabels
			tcp.Spec.ControlPlane.Service.AdditionalMetadata.Annotations = scp.Spec.Network.ServiceAnnotations

			for _, i := range scp.Spec.Network.CertSANs {
				// validating CertSANs as soon as possible to avoid github.com/butlerdotdev/steward/issues/679:
				// nil err means the entry is in the form of <HOST>:<PORT> which is not accepted
				if _, _, err := net.SplitHostPort(i); err == nil {
					return errors.Wrap(ErrUnsupportedCertificateSAN, fmt.Sprintf("entry %s is invalid", i))
				}
			}

			tcp.Spec.NetworkProfile.CertSANs = scp.Spec.Network.CertSANs
			// Ingress
			if scp.Spec.Network.Ingress != nil {
				tcp.Spec.ControlPlane.Ingress = &stewardv1alpha1.IngressSpec{
					AdditionalMetadata: stewardv1alpha1.AdditionalMetadata{
						Labels:      scp.Spec.Network.Ingress.ExtraLabels,
						Annotations: scp.Spec.Network.Ingress.ExtraAnnotations,
					},
					IngressClassName: scp.Spec.Network.Ingress.ClassName,
					Hostname:         scp.Spec.Network.Ingress.Hostname,
				}
				// In the case of enabled ingress, adding the FQDN to the CertSANs
				if tcp.Spec.NetworkProfile.CertSANs == nil {
					tcp.Spec.NetworkProfile.CertSANs = []string{}
				}

				if host, _, err := net.SplitHostPort(scp.Spec.Network.Ingress.Hostname); err == nil {
					// no error means <FQDN>:<PORT>, we need the host variable
					tcp.Spec.NetworkProfile.CertSANs = append(tcp.Spec.NetworkProfile.CertSANs, host)
				} else {
					// No port specification, adding bare entry
					tcp.Spec.NetworkProfile.CertSANs = append(tcp.Spec.NetworkProfile.CertSANs, scp.Spec.Network.Ingress.Hostname)
				}
			} else {
				tcp.Spec.ControlPlane.Ingress = nil
			}
			// LoadBalancer
			if scp.Spec.Network.LoadBalancerConfig != nil {
				if lbClass := scp.Spec.Network.LoadBalancerConfig.LoadBalancerClass; lbClass != nil {
					tcp.Spec.NetworkProfile.LoadBalancerClass = ptr.To(*lbClass)
				}

				if srcRange := scp.Spec.Network.LoadBalancerConfig.LoadBalancerSourceRanges; srcRange != nil {
					tcp.Spec.NetworkProfile.LoadBalancerSourceRanges = srcRange
				}
			}

			// Deployment
			tcp.Spec.ControlPlane.Deployment.NodeSelector = scp.Spec.Deployment.NodeSelector
			tcp.Spec.ControlPlane.Deployment.RuntimeClassName = scp.Spec.Deployment.RuntimeClassName
			tcp.Spec.ControlPlane.Deployment.ServiceAccountName = scp.Spec.Deployment.ServiceAccountName
			tcp.Spec.ControlPlane.Deployment.AdditionalMetadata = scp.Spec.Deployment.AdditionalMetadata
			tcp.Spec.ControlPlane.Deployment.PodAdditionalMetadata = scp.Spec.Deployment.PodAdditionalMetadata
			tcp.Spec.ControlPlane.Deployment.Strategy = scp.Spec.Deployment.Strategy
			tcp.Spec.ControlPlane.Deployment.Affinity = scp.Spec.Deployment.Affinity
			tcp.Spec.ControlPlane.Deployment.Tolerations = scp.Spec.Deployment.Tolerations
			tcp.Spec.ControlPlane.Deployment.TopologySpreadConstraints = scp.Spec.Deployment.TopologySpreadConstraints
			tcp.Spec.ControlPlane.Deployment.AdditionalInitContainers = scp.Spec.Deployment.ExtraInitContainers
			tcp.Spec.ControlPlane.Deployment.AdditionalContainers = scp.Spec.Deployment.ExtraContainers
			tcp.Spec.ControlPlane.Deployment.AdditionalVolumes = scp.Spec.Deployment.ExtraVolumes

			if !isDelegatedExternally {
				return controllerutil.SetControllerReference(&scp, tcp, k8sClient.Scheme())
			}

			return nil
		})

		return scopeErr //nolint:wrapcheck
	})
	if err != nil {
		return nil, errors.Wrap(err, "cannot create or update TenantControlPlane")
	}

	return tcp, nil
}
