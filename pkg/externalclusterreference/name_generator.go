// Copyright 2025 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package externalclusterreference

import (
	"strings"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"

	"github.com/butlerdotdev/cluster-api-control-plane-provider-steward/api/v1alpha1"
)

const (
	RemoteTCPPrefix = "kcp-"
)

func ParseStewardControlPlaneUIDFromTenantControlPlane(tcp stewardv1alpha1.TenantControlPlane) string {
	if !strings.HasPrefix(tcp.Name, RemoteTCPPrefix) {
		return ""
	}

	return strings.TrimPrefix(tcp.Name, RemoteTCPPrefix)
}

func GenerateRemoteTenantControlPlaneNames(kcp v1alpha1.StewardControlPlane) (name string, namespace string) { //nolint:nonamedreturns
	return RemoteTCPPrefix + string(kcp.UID), kcp.Spec.Deployment.ExternalClusterReference.DeploymentNamespace
}

func GenerateKeyNameFromSecret(secret *corev1.Secret) []string {
	names := make([]string, 0, len(secret.Data))

	for k := range secret.Data {
		names = append(names, secret.Namespace+"/"+secret.Name+"/"+k)
	}

	return names
}

func GenerateKeyNameFromSteward(kcp *v1alpha1.StewardControlPlane) string {
	namespace := kcp.Namespace

	if kcp.Spec.Deployment.ExternalClusterReference.KubeconfigSecretNamespace != "" {
		namespace = kcp.Spec.Deployment.ExternalClusterReference.KubeconfigSecretNamespace
	}

	return namespace + "/" + kcp.Spec.Deployment.ExternalClusterReference.KubeconfigSecretName + "/" + kcp.Spec.Deployment.ExternalClusterReference.KubeconfigSecretKey
}
