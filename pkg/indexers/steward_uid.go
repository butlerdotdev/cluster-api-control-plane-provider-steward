// Copyright 2025 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package indexers

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	scpv1alpha1 "github.com/butlerdotdev/cluster-api-control-plane-provider-steward/api/v1alpha1"
)

const (
	StewardControlPlaneUIDField = "stewardControlPlaneUID"
)

type StewardControlPlaneUID struct{}

func (k StewardControlPlaneUID) Object() client.Object { //nolint:ireturn
	return &scpv1alpha1.StewardControlPlane{}
}

func (k StewardControlPlaneUID) Field() string {
	return StewardControlPlaneUIDField
}

func (k StewardControlPlaneUID) ExtractValue() client.IndexerFunc {
	return func(object client.Object) []string {
		kcp := object.(*scpv1alpha1.StewardControlPlane) //nolint:forcetypeassert

		return []string{string(kcp.UID)}
	}
}
