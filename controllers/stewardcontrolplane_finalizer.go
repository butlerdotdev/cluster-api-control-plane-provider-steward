// Copyright 2025 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/butlerdotdev/cluster-api-control-plane-provider-steward/api/v1alpha1"
	"github.com/butlerdotdev/cluster-api-control-plane-provider-steward/pkg/externalclusterreference"
)

func (r *StewardControlPlaneReconciler) handleFinalizer(ctx context.Context, scp *v1alpha1.StewardControlPlane) error {
	finalizers := sets.New[string](scp.Finalizers...)
	if !finalizers.Has(ExternalClusterReferenceFinalizer) {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() (scopedErr error) { //nolint:nonamedreturns
			if scopedErr = r.client.Get(ctx, types.NamespacedName{Namespace: scp.Namespace, Name: scp.Name}, scp); scopedErr != nil {
				return scopedErr //nolint:wrapcheck
			}

			finalizers.Insert(ExternalClusterReferenceFinalizer)

			scp.SetFinalizers(finalizers.UnsortedList())

			return r.client.Update(ctx, scp)
		})
		if err != nil {
			return err //nolint:wrapcheck
		}
	}

	return nil
}

func (r *StewardControlPlaneReconciler) handleDeletion(ctx context.Context, scp v1alpha1.StewardControlPlane) error {
	finalizers, log := sets.New[string](scp.Finalizers...), ctrllog.FromContext(ctx)

	if !finalizers.Has(ExternalClusterReferenceFinalizer) || scp.Spec.Deployment.ExternalClusterReference == nil {
		log.Info("waiting for StewardControlPlane finalizers")

		return nil
	}

	remoteClient, cErr := r.extractRemoteClient(ctx, scp)
	if cErr != nil {
		log.Error(cErr, "cannot generate remote client for deletion")

		return cErr
	}

	var tcp stewardv1alpha1.TenantControlPlane
	tcp.Name, tcp.Namespace = externalclusterreference.GenerateRemoteTenantControlPlaneNames(scp)

	if tcpErr := remoteClient.Delete(ctx, &tcp); tcpErr != nil {
		if errors.IsNotFound(tcpErr) {
			log.Info("remote TenantControlPlane is already deleted")

			return nil
		}

		log.Error(tcpErr, "cannot delete remote TenantControlPlane")

		return tcpErr //nolint:wrapcheck
	}

	log.Info("remote TenantControlPlane has been deleted")

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := r.client.Get(ctx, types.NamespacedName{Name: scp.Name, Namespace: scp.Namespace}, &scp); err != nil {
			return err //nolint:wrapcheck
		}

		finalizers = sets.New[string](scp.Finalizers...)
		finalizers.Delete(ExternalClusterReferenceFinalizer)

		scp.Finalizers = finalizers.UnsortedList()

		return r.client.Update(ctx, &scp)
	})
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("object may have been deleted")

			return nil
		}

		log.Error(err, "unable to remove finalizer")

		return err //nolint:wrapcheck
	}

	log.Info("finalizer has been removed")

	return nil
}
