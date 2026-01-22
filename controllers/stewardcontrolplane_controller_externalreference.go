// Copyright 2025 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/butlerdotdev/cluster-api-control-plane-provider-steward/api/v1alpha1"
	ecr "github.com/butlerdotdev/cluster-api-control-plane-provider-steward/pkg/externalclusterreference"
	"github.com/butlerdotdev/cluster-api-control-plane-provider-steward/pkg/features"
)

const (
	ExternalClusterReferenceFinalizer = "ecr.steward.butlerlabs.dev/finalizer"
)

var (
	ErrExternalClusterReferenceNotEnabled                 = errors.New("external cluster feature gates are not enabled")
	ErrExternalClusterReferenceCrossNamespaceReference    = errors.New("the ExternalClusterReference is enforcing kubeconfig in the same Namespace, ExternalClusterReferenceCrossNamespace must be enabled")
	ErrExternalCLusterReferenceSecretEmptyError           = errors.New("could not extract kubeconfig for external cluster reference, secret is empty")
	ErrExternalClusterReferenceSecretKeyEmpty             = errors.New("could not extract kubeconfig for external cluster reference, key is empty")
	ErrExternalClusterReferenceNonInitializedStore        = errors.New("remote manager is not yet initialized")
	ErrExternalClusterReferenceTenantControlPlaneNotFound = errors.New("TenantControlPlane custom resource not available in external cluster")
)

//nolint:cyclop
func (r *StewardControlPlaneReconciler) extractRemoteClient(ctx context.Context, scp v1alpha1.StewardControlPlane) (client.Client, error) { //nolint:ireturn
	if !r.FeatureGates.Enabled(features.ExternalClusterReference) {
		return nil, ErrExternalClusterReferenceNotEnabled
	}

	if r.FeatureGates.Enabled(features.ExternalClusterReference) &&
		!r.FeatureGates.Enabled(features.ExternalClusterReferenceCrossNamespace) &&
		scp.Spec.Deployment.ExternalClusterReference.KubeconfigSecretNamespace != "" &&
		scp.Spec.Deployment.ExternalClusterReference.KubeconfigSecretNamespace != scp.Namespace {
		return nil, ErrExternalClusterReferenceCrossNamespaceReference
	}

	namespace := scp.Namespace

	if scp.Spec.Deployment.ExternalClusterReference.KubeconfigSecretNamespace != "" {
		namespace = scp.Spec.Deployment.ExternalClusterReference.KubeconfigSecretNamespace
	}

	var secret corev1.Secret

	if err := r.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: scp.Spec.Deployment.ExternalClusterReference.KubeconfigSecretName}, &secret); err != nil {
		return nil, errors.Wrapf(err, "could not get external cluster reference secret")
	}

	if secret.Data == nil {
		return nil, ErrExternalCLusterReferenceSecretEmptyError
	}

	if secret.Data[scp.Spec.Deployment.ExternalClusterReference.KubeconfigSecretKey] == nil {
		return nil, ErrExternalClusterReferenceSecretKeyEmpty
	}

	mgr, found := r.ExternalClusterReferenceStore.Get(ecr.GenerateKeyNameFromSteward(&scp), secret.ResourceVersion)
	if !found {
		return nil, ErrExternalClusterReferenceNonInitializedStore
	}

	// Use the RESTMapper to check if the CRD is installed
	gvr := stewardv1alpha1.GroupVersion.WithResource("tenantcontrolplanes")
	if _, err := mgr.GetRESTMapper().KindFor(gvr); err != nil {
		return nil, ErrExternalClusterReferenceTenantControlPlaneNotFound
	}

	return mgr.GetClient(), nil
}
