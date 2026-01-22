// Copyright 2025 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package features

const (
	// ExternalClusterReference allows deploying Tenant Control Plane pods to a different cluster from the Management one.
	// This will require a valid kubeconfig referenced in the StewardControlPlane object, in the same Namespace of the said object.
	ExternalClusterReference = "ExternalClusterReference"

	// ExternalClusterReferenceCrossNamespace allows deploying Tenant Control Plane pods to a different cluster from the Management one.
	// It supports referencing a kubeconfig available in a different Namespace than the StewardControlPlane.
	ExternalClusterReferenceCrossNamespace = "ExternalClusterReferenceCrossNamespace"

	// SkipInfraClusterPatch bypasses patching the InfraCluster with the control-plane endpoint.
	SkipInfraClusterPatch = "SkipInfraClusterPatch"

	// DynamicInfrastructureClusterPatch allows patching any generic InfraCluster with the control-plane endpoint
	// provided by Steward.
	DynamicInfrastructureClusterPatch = "DynamicInfrastructureClusterPatch"
)
