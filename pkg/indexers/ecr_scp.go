// Copyright 2025 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package indexers

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	scpv1alpha1 "github.com/butlerdotdev/cluster-api-control-plane-provider-steward/api/v1alpha1"
	ecr "github.com/butlerdotdev/cluster-api-control-plane-provider-steward/pkg/externalclusterreference"
)

const (
	ExternalClusterReferenceStewardControlPlaneField = "externalClusterReferenceStewardControlPlane"
)

type ExternalClusterReferenceStewardControlPlane struct{}

func (e ExternalClusterReferenceStewardControlPlane) Object() client.Object { //nolint:ireturn
	return &scpv1alpha1.StewardControlPlane{}
}

func (e ExternalClusterReferenceStewardControlPlane) Field() string {
	return ExternalClusterReferenceStewardControlPlaneField
}

func (e ExternalClusterReferenceStewardControlPlane) ExtractValue() client.IndexerFunc {
	return func(object client.Object) []string {
		kcp := object.(*scpv1alpha1.StewardControlPlane) //nolint:forcetypeassert

		if kcp.Spec.Deployment.ExternalClusterReference != nil {
			return []string{ecr.GenerateKeyNameFromSteward(kcp)}
		}

		return nil
	}
}
