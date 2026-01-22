// Copyright 2025 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

type StewardControlPlaneConditionType string

var (
	FoundExternalClusterReferenceConditionType  StewardControlPlaneConditionType = "FoundExternalReferenceClient"
	TenantControlPlaneCreatedConditionType      StewardControlPlaneConditionType = "TenantControlPlaneCreated"
	TenantControlPlaneAddressReadyConditionType StewardControlPlaneConditionType = "TenantControlPlaneAddressReady"
	ControlPlaneEndpointPatchedConditionType    StewardControlPlaneConditionType = "ControlPlaneEndpointPatched"
	InfrastructureClusterPatchedConditionType   StewardControlPlaneConditionType = "InfrastructureClusterPatched"
	StewardControlPlaneInitializedConditionType  StewardControlPlaneConditionType = "StewardControlPlaneIsInitialized"
	StewardControlPlaneReadyConditionType        StewardControlPlaneConditionType = "StewardControlPlaneIsReady"
	KubeadmResourcesCreatedReadyConditionType   StewardControlPlaneConditionType = "KubeadmResourcesCreated"
)
