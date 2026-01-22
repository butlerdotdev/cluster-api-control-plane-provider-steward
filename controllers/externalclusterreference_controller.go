// Copyright 2025 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"strings"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/butlerdotdev/cluster-api-control-plane-provider-steward/api/v1alpha1"
	"github.com/butlerdotdev/cluster-api-control-plane-provider-steward/pkg/externalclusterreference"
	"github.com/butlerdotdev/cluster-api-control-plane-provider-steward/pkg/indexers"
)

type ExternalClusterReferenceReconciler struct {
	Client         client.Client
	Store          externalclusterreference.Store
	TriggerChannel chan event.GenericEvent
}

//nolint:funlen,cyclop
func (r *ExternalClusterReferenceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	var secret corev1.Secret
	if err := r.Client.Get(ctx, req.NamespacedName, &secret); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		log.Error(err, "unable to fetch Secret")

		return ctrl.Result{}, err //nolint:wrapcheck
	}
	//nolint:prealloc
	var keys []string

	for _, key := range externalclusterreference.GenerateKeyNameFromSecret(&secret) {
		var scpList v1alpha1.StewardControlPlaneList

		if err := r.Client.List(ctx, &scpList, client.MatchingFields{indexers.ExternalClusterReferenceStewardControlPlaneField: key}); err != nil {
			log.Error(err, "unable to use indexer", "key", key)

			return ctrl.Result{}, err //nolint:wrapcheck
		}

		if len(scpList.Items) == 0 {
			if r.Store.Stop(key) {
				log.Info("stopping manager, unused")
			}

			continue
		}

		log.Info("secret entry is referenced", "key", key, "count", len(scpList.Items))

		keys = append(keys, key)
	}

	for _, key := range keys {
		if _, found := r.Store.Get(key, secret.ResourceVersion); found {
			continue
		}

		if !r.Store.Stop(key) {
			log.Info("new configuration, loading manager")
		} else {
			log.Info("configuration seems changed, restarting manager")
		}

		cfg, cfgErr := clientcmd.RESTConfigFromKubeConfig(secret.Data[strings.Split(key, "/")[2]])
		if cfgErr != nil {
			log.Error(cfgErr, "cannot generate REST config from Secret content", "key", key)

			return ctrl.Result{}, cfgErr //nolint:wrapcheck
		}

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme:  r.Client.Scheme(),
			Metrics: server.Options{BindAddress: "0"},
			Cache: cache.Options{
				ByObject: map[client.Object]cache.ByObject{
					// Reduce memory overhead by only caching watched resources.
					&stewardv1alpha1.TenantControlPlane{}: {},
				},
			},
		})
		if err != nil {
			log.Error(err, "cannot generate manager")

			return ctrl.Result{}, err //nolint:wrapcheck
		}

		if err = (&PushStewardChange{ParentClient: r.Client, Client: mgr.GetClient(), TriggerChannel: r.TriggerChannel}).SetupWithManager(mgr); err != nil {
			log.Error(err, "unable to create controller", "controller", "PushStewardChange")

			return ctrl.Result{}, err
		}

		mgrCtx, cancelFn := context.WithCancel(ctx)
		go r.startManager(mgrCtx, mgr, key)

		r.Store.Add(key, secret.ResourceVersion, mgr, cancelFn)
	}

	return ctrl.Result{}, nil
}

func (r *ExternalClusterReferenceReconciler) startManager(ctx context.Context, mgr ctrl.Manager, name string) {
	if mgrErr := mgr.Start(ctx); mgrErr != nil {
		ctrllog.FromContext(ctx).Error(mgrErr, "manager cannot be started, external cluster reference could not work")

		r.Store.Stop(name)
	}
}

func (r *ExternalClusterReferenceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	//nolint:wrapcheck
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		Watches(&v1alpha1.StewardControlPlane{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, object client.Object) []reconcile.Request {
			scp := object.(*v1alpha1.StewardControlPlane) //nolint:forcetypeassert
			if scp.Spec.Deployment.ExternalClusterReference == nil {
				return nil
			}

			var requests []reconcile.Request

			for _, secret := range r.getSecretFromStewardControlPlaneReferences(ctx, scp) {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: secret.Namespace,
						Name:      secret.Name,
					},
				})
			}

			return requests
		})).
		Complete(r)
}

func (r *ExternalClusterReferenceReconciler) getSecretFromStewardControlPlaneReferences(ctx context.Context, scp *v1alpha1.StewardControlPlane) []corev1.Secret {
	var secretList corev1.SecretList

	val := externalclusterreference.GenerateKeyNameFromSteward(scp)

	if err := r.Client.List(ctx, &secretList, client.MatchingFields{indexers.ExternalClusterReferenceSecretField: val}); err != nil {
		return nil
	}

	return secretList.Items
}

type PushStewardChange struct {
	ParentClient   client.Client
	Client         client.Client
	TriggerChannel chan event.GenericEvent
}

func (p *PushStewardChange) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	var tcp stewardv1alpha1.TenantControlPlane

	if err := p.Client.Get(ctx, request.NamespacedName, &tcp); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, err //nolint:wrapcheck
	}

	value := externalclusterreference.ParseStewardControlPlaneUIDFromTenantControlPlane(tcp)
	if value == "" {
		return reconcile.Result{}, nil
	}

	var scpList v1alpha1.StewardControlPlaneList

	if err := p.ParentClient.List(ctx, &scpList, client.MatchingFields{indexers.StewardControlPlaneUIDField: value}); err != nil {
		return reconcile.Result{}, err //nolint:wrapcheck
	}

	for _, scp := range scpList.Items {
		p.TriggerChannel <- event.GenericEvent{
			Object: &v1alpha1.StewardControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      scp.Name,
					Namespace: scp.Namespace,
				},
			},
		}
	}

	return reconcile.Result{}, nil
}

//nolint:wrapcheck
func (p *PushStewardChange) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			SkipNameValidation: ptr.To(true),
		}).
		For(&stewardv1alpha1.TenantControlPlane{}).
		Complete(p)
}
