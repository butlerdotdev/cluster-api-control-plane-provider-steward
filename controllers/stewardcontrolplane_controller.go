// Copyright 2025 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"time"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
	goerrors "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/component-base/featuregate"
	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	scpv1alpha1 "github.com/butlerdotdev/cluster-api-control-plane-provider-steward/api/v1alpha1"
	"github.com/butlerdotdev/cluster-api-control-plane-provider-steward/pkg/externalclusterreference"
	"github.com/butlerdotdev/cluster-api-control-plane-provider-steward/pkg/features"
)

// StewardControlPlaneReconciler reconciles a StewardControlPlane object.
type StewardControlPlaneReconciler struct {
	ExternalClusterReferenceStore externalclusterreference.Store
	FeatureGates                  featuregate.FeatureGate
	MaxConcurrentReconciles       int
	DynamicInfrastructureClusters sets.Set[string]

	client client.Client
}

//+kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=stewardcontrolplanes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=stewardcontrolplanes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=stewardcontrolplanes/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch

func (r *StewardControlPlaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) { //nolint:funlen,cyclop,maintidx,gocognit,gocyclo
	var err error

	now, log := time.Now(), ctrllog.FromContext(ctx)

	log.Info("reconciliation started")

	// Retrieving the StewardControlPlane instance from the request
	scp := scpv1alpha1.StewardControlPlane{}
	if err = r.client.Get(ctx, req.NamespacedName, &scp); err != nil {
		if errors.IsNotFound(err) {
			log.Info("resource may have been deleted")

			return ctrl.Result{}, nil
		}

		log.Error(err, "unable to get scpv1alpha1.StewardControlPlane")

		return ctrl.Result{}, err //nolint:wrapcheck
	}
	// The ControlPlane must have an OwnerReference set from the Cluster controller, waiting for this condition:
	// https://cluster-api.sigs.k8s.io/developer/architecture/controllers/control-plane.html#relationship-to-other-cluster-api-types
	if len(scp.GetOwnerReferences()) == 0 {
		log.Info("missing OwnerReference from the Cluster controller, waiting for it")

		return ctrl.Result{}, nil
	}

	// Retrieving the Cluster information
	cluster := capiv1beta1.Cluster{}
	cluster.SetName(scp.GetOwnerReferences()[0].Name)
	cluster.SetNamespace(scp.GetNamespace())

	if err = r.client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, &cluster); err != nil {
		if errors.IsNotFound(err) {
			log.Info("capiv1beta1.Cluster resource may have been deleted, withdrawing reconciliation")

			return ctrl.Result{}, nil
		}

		log.Error(err, "unable to get capiv1beta1.Cluster")

		return ctrl.Result{}, err //nolint:wrapcheck
	}

	// Return early if the object or Cluster is paused.
	if annotations.IsPaused(&cluster, &scp) {
		log.Info("Reconciliation is paused for this object")

		return ctrl.Result{}, nil
	}

	// Handling finalizer for external deployment:
	// in case of ExternalClusterReference the remote TCP must be deleted.
	if scp.DeletionTimestamp != nil {
		return ctrl.Result{}, r.handleDeletion(ctx, scp)
	}

	// Extracting conditions, used to update the StewardControlPlane ones upon the end of the reconciliation.
	conditions := scp.Status.Conditions

	defer func() {
		deferErr := r.updateStewardControlPlaneStatus(ctx, &scp, func() {
			scp.Status.Conditions = conditions
		})

		if deferErr != nil {
			log.Error(err, "unable to update scpv1alpha1.StewardControlPlane conditions")
		}
	}()
	// When ExternalClusterReference feature is enabled, we need to interact with a different API endpoint
	// to deploy and read the resulting Tenant Control Plane: in the case of nil value, it means we're targeting
	// the same management cluster, so no extra quirks are required.
	var remoteClient client.Client

	if scp.Spec.Deployment.ExternalClusterReference != nil {
		TrackConditionType(&conditions, scpv1alpha1.FoundExternalClusterReferenceConditionType, scp.Generation, func() error {
			remoteClient, err = r.extractRemoteClient(ctx, scp)

			return err
		})

		if err != nil {
			log.Error(err, "unable to get remote Client")

			return ctrl.Result{}, err
		}

		if err = r.handleFinalizer(ctx, &scp); err != nil {
			log.Error(err, "unable to update finalizers")

			return ctrl.Result{}, err
		}
	}
	// Reconciling the Steward TenantControlPlane resource
	var tcp *stewardv1alpha1.TenantControlPlane

	TrackConditionType(&conditions, scpv1alpha1.TenantControlPlaneCreatedConditionType, scp.Generation, func() error {
		tcp, err = r.createOrUpdateTenantControlPlane(ctx, remoteClient, cluster, scp)

		return err
	})

	if err != nil {
		log.Error(err, "unable to create or update the TenantControlPlane instance")

		return ctrl.Result{}, err
	}
	// Waiting for the TenantControlPlane address: pay attention!
	//
	// This is still a work-in-progress and changing the Control Plane Controller contract.
	// Due to the given for granted concept that Control Plane and Worker nodes are on the same infrastructure,
	// we have to change the approach and wait for the advertised Control Plane endpoint, since Steward is offering a
	// Managed Kubernetes Service, although running as a regular pod.
	TrackConditionType(&conditions, scpv1alpha1.TenantControlPlaneAddressReadyConditionType, scp.Generation, func() error {
		if len(tcp.Status.ControlPlaneEndpoint) == 0 {
			err = goerrors.New("Control Plane Endpoint is not yet available since unprocessed by Steward")
		}

		return err
	})
	// Treating the missing Control Plane Endpoint error as a sentinel:
	// there's no need to start the requeue with error logging, the Infrastructure Provider will react once the address
	// is available and assigned to the managed TenantControlPlane resource.
	if err != nil {
		log.Info(err.Error() + ", enqueuing back")

		return ctrl.Result{RequeueAfter: time.Second}, nil
	}
	// Starting from CAPI v1.8, the ControlPlane provider can set the Control Plane endpoint:
	// this will make useless the patchCluster function in the future.
	// More info: https://release-1-8.cluster-api.sigs.k8s.io/developer/providers/control-plane#optional-spec-fields-for-implementations-providing-endpoints
	TrackConditionType(&conditions, scpv1alpha1.ControlPlaneEndpointPatchedConditionType, scp.Generation, func() error {
		err = r.patchControlPlaneEndpoint(ctx, &scp, tcp.Status.ControlPlaneEndpoint)

		return err
	})

	if err != nil {
		log.Error(err, "cannot patch scpv1alpha1.StewardControlPlane")

		return ctrl.Result{}, err
	}

	// We need to fetch the updated cluster resource here because otherwise the cluster.spec.controlPlaneEndpoint.Host
	// check that happens latter will never succeed.
	if err = r.client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, &cluster); err != nil {
		if errors.IsNotFound(err) {
			log.Info("capiv1beta1.Cluster resource may have been deleted, withdrawing reconciliation")

			return ctrl.Result{}, nil
		}

		log.Error(err, "unable to get capiv1beta1.Cluster")

		return ctrl.Result{}, err //nolint:wrapcheck
	}

	// The following code path will be skipped when the InfraClusterOptional=true. This enables
	// the use of a StewardControlPlane without an infrastructure cluster.
	if !r.FeatureGates.Enabled(features.SkipInfraClusterPatch) {
		// Patching the Infrastructure Cluster:
		// this will be removed on the upcoming Steward Control Plane versions.
		TrackConditionType(&conditions, scpv1alpha1.InfrastructureClusterPatchedConditionType, scp.Generation, func() error {
			err = r.patchCluster(ctx, cluster, &scp, tcp.Status.ControlPlaneEndpoint)

			return err
		})

		if err != nil {
			log.Error(err, "cannot patch capiv1beta1.Cluster")

			return ctrl.Result{}, err
		}
	}

	// Before continuing, the Cluster object needs some validation, such as:
	// 1. an assigned Control Plane endpoint
	// 2. a ready infrastructure
	if len(cluster.Spec.ControlPlaneEndpoint.Host) == 0 {
		log.Info("capiv1beta1.Cluster Control Plane endpoint still unprocessed, enqueuing back")

		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	if !cluster.Status.InfrastructureReady {
		log.Info("capiv1beta1.Cluster infrastructure is not yet ready, enqueuing back")

		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	if tcp.Status.Kubernetes.Version.Status == nil {
		log.Info("stewardv1alpha1.TenantControlPlane is not yet initialized, enqueuing back")

		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	if *tcp.Status.Kubernetes.Version.Status == stewardv1alpha1.VersionReady && !scp.Status.Initialized {
		// TenantControlPlane has been initialized
		TrackConditionType(&conditions, scpv1alpha1.StewardControlPlaneInitializedConditionType, scp.Generation, func() error {
			err = r.updateStewardControlPlaneStatus(ctx, &scp, func() {
				scp.Status.Initialized = true
			})

			return err
		})

		if err != nil {
			log.Error(err, "unable to set scpv1alpha1.StewardControlPlane as initialized")

			return ctrl.Result{}, err
		}
	}

	if !scp.Status.Initialized {
		log.Info("scpv1alpha1.StewardControlPlane is not yet initialized, enqueuing back")

		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	// Updating StewardControlPlane ready status, along with scaling values
	TrackConditionType(&conditions, scpv1alpha1.StewardControlPlaneInitializedConditionType, scp.Generation, func() error {
		err = r.updateStewardControlPlaneStatus(ctx, &scp, func() {
			scp.Status.ReadyReplicas = tcp.Status.Kubernetes.Deployment.ReadyReplicas
			scp.Status.Replicas = tcp.Status.Kubernetes.Deployment.Replicas
			scp.Status.Selector = metav1.FormatLabelSelector(&metav1.LabelSelector{MatchLabels: scp.GetLabels()})
			scp.Status.UnavailableReplicas = tcp.Status.Kubernetes.Deployment.UnavailableReplicas
			scp.Status.UpdatedReplicas = tcp.Status.Kubernetes.Deployment.UpdatedReplicas
			scp.Status.Version = tcp.Status.Kubernetes.Version.Version
		})

		return err
	})

	if err != nil {
		log.Error(err, "unable to report scpv1alpha1.StewardControlPlane status")

		return ctrl.Result{}, err
	}
	// StewardControlPlane must be considered ready before replicating required resources
	TrackConditionType(&conditions, scpv1alpha1.StewardControlPlaneInitializedConditionType, scp.Generation, func() error {
		err = r.updateStewardControlPlaneStatus(ctx, &scp, func() {
			scp.Status.Initialized = true
		})

		return err
	})

	var result ctrl.Result

	TrackConditionType(&conditions, scpv1alpha1.KubeadmResourcesCreatedReadyConditionType, scp.Generation, func() error {
		err = r.createRequiredResources(ctx, remoteClient, cluster, scp, tcp)

		return err
	})

	if err != nil {
		if goerrors.Is(err, ErrEnqueueBack) {
			log.Info(err.Error())

			return ctrl.Result{RequeueAfter: time.Second}, nil
		}

		log.Error(err, "unable to satisfy Secrets contract")

		return ctrl.Result{}, err
	}

	TrackConditionType(&conditions, scpv1alpha1.StewardControlPlaneReadyConditionType, scp.Generation, func() error {
		err = r.updateStewardControlPlaneStatus(ctx, &scp, func() {
			scp.Status.Ready = *tcp.Status.Kubernetes.Version.Status == stewardv1alpha1.VersionReady || *tcp.Status.Kubernetes.Version.Status == stewardv1alpha1.VersionUpgrading
		})
		if err != nil {
			return err
		}

		if !scp.Status.Ready {
			return fmt.Errorf("TenantControlPlane in %s status, %w", *tcp.Status.Kubernetes.Version.Status, ErrEnqueueBack)
		}

		return nil
	})

	if err != nil {
		if goerrors.Is(err, ErrEnqueueBack) {
			log.Info(err.Error())

			return ctrl.Result{RequeueAfter: time.Second}, nil
		}

		log.Error(err, "unable to report scpv1alpha1.StewardControlPlane readiness")

		return ctrl.Result{}, err
	}

	log.Info("reconciliation completed", "duration", time.Since(now).String())

	return result, nil
}

func (r *StewardControlPlaneReconciler) updateStewardControlPlaneStatus(ctx context.Context, scp *scpv1alpha1.StewardControlPlane, modifierFn func()) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := r.client.Get(ctx, types.NamespacedName{Name: scp.Name, Namespace: scp.Namespace}, scp); err != nil {
			return err //nolint:wrapcheck
		}

		modifierFn()

		return r.client.Status().Update(ctx, scp)
	})
	if err != nil {
		return goerrors.Wrap(err, "cannot update StewardControlPlane resource")
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *StewardControlPlaneReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, channel chan event.GenericEvent) error {
	r.client = mgr.GetClient()
	ctrlBuilder := ctrl.NewControllerManagedBy(mgr).
		For(&scpv1alpha1.StewardControlPlane{}, builder.WithPredicates(predicate.NewPredicateFuncs(func(object client.Object) bool {
			return len(object.GetOwnerReferences()) > 0
		}))).
		Owns(&corev1.Secret{}).
		WatchesRawSource(source.Channel(channel, &handler.EnqueueRequestForObject{})).
		WithOptions(controller.Options{MaxConcurrentReconciles: r.MaxConcurrentReconciles}).
		WithEventFilter(predicates.ResourceNotPaused(mgr.GetScheme(), ctrl.LoggerFrom(ctx)))

	cs, csErr := kubernetes.NewForConfig(mgr.GetConfig())
	if csErr != nil {
		return goerrors.Wrap(csErr, "cannot create Kubernetes Client-set")
	}

	if _, rsErr := cs.Discovery().ServerResourcesForGroupVersion(stewardv1alpha1.GroupVersion.String()); rsErr == nil {
		ctrlBuilder = ctrlBuilder.Owns(&stewardv1alpha1.TenantControlPlane{})
	}

	//nolint:wrapcheck
	return ctrlBuilder.Complete(r)
}
