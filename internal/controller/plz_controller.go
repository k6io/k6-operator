/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/grafana/k6-operator/api/v1alpha1"
	k6v1alpha1 "github.com/grafana/k6-operator/api/v1alpha1"
	"github.com/grafana/k6-operator/pkg/cloud"
)

const (
	plzFinalizer     = "privateloadzones.k6.io/finalizer"
	plzUIDAnnotation = "privateloadzones.k6.io/plz-uid"
)

// PrivateLoadZoneReconciler reconciles a PrivateLoadZone object
type PrivateLoadZoneReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	// Note: we expect that there's only one PLZ object at a time.
	// Therefore it is safe to assume that poller should be created only once
	// and it can simply be part of the Reconciler object.
	// If support for multiple PLZs will be added at some point,
	// poller should be made PLZ specific;
	// e.g. with a map: PLZ name -> poller.
	poller *cloud.TestRunPoller
	token  string // needed for cloud logs
}

//+kubebuilder:rbac:groups=k6.io,resources=privateloadzones,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k6.io,resources=privateloadzones/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k6.io,resources=privateloadzones/finalizers,verbs=get;update;patch

// Reconcile takes a PrivateLoadZone object and takes the appropriate action in the cluster
func (r *PrivateLoadZoneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("namespace", req.Namespace, "name", req.Name, "reconcileID", controller.ReconcileIDFromContext(ctx))

	plz := &v1alpha1.PrivateLoadZone{}
	err := r.Get(ctx, req.NamespacedName, plz)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			logger.Info("Request deleted. Nothing to reconcile.")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Could not fetch request")
		return ctrl.Result{Requeue: true}, err
	}

	if r.poller == nil {
		token, tokenReady, err := loadToken(ctx, logger, r.Client, plz.Spec.Token, &client.ListOptions{Namespace: plz.Namespace})
		if err != nil {
			// An error here means a very likely mis-configuration of the token.
			logger.Error(err, "A problem while getting token.")
			return ctrl.Result{}, nil
		}
		if !tokenReady {
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}

		r.poller = cloud.NewTestRunPoller(cloud.ApiURL(k6CloudHost()), token, plz.Name, logger)
		r.token = token
	}

	if plz.DeletionTimestamp.IsZero() && (plz.IsUnknown(v1alpha1.PLZRegistered) || plz.IsFalse(v1alpha1.PLZRegistered)) {
		if controllerutil.ContainsFinalizer(plz, plzFinalizer) {
			// PLZ has been already registered so just update status accordingly

			plz.Initialize()
			plz.UpdateCondition(v1alpha1.PLZRegistered, metav1.ConditionTrue)

			if _, err := r.UpdateStatus(ctx, plz, logger); err != nil {
				return ctrl.Result{}, err
			}
		} else {
			// This is the first reconcile: PLZ should be registered
			uid, err := plz.Register(ctx, logger, r.poller.Client)
			if err != nil {
				return ctrl.Result{}, err
			}

			logger.Info(fmt.Sprintf("PLZ %s is registered with k6 Cloud.", plz.Name))

			controllerutil.AddFinalizer(plz, plzFinalizer)
			plz.SetAnnotations(map[string]string{
				plzUIDAnnotation: uid,
			})

			if err := r.Update(ctx, plz); err != nil {
				return ctrl.Result{}, err
			}

			return ctrl.Result{Requeue: true}, nil
		}
	} else {
		if !plz.DeletionTimestamp.IsZero() && controllerutil.ContainsFinalizer(plz, plzFinalizer) {
			// PLZ has been deleted.

			r.poller.Stop()

			// Since resource is being deleted, there isn't much to do about
			// deregistration error here.
			_ = plz.Deregister(ctx, logger, r.poller.Client)

			logger.Info(fmt.Sprintf("PLZ %s is deregistered with k6 Cloud.", plz.Name))

			controllerutil.RemoveFinalizer(plz, plzFinalizer)

			if err := r.Update(ctx, plz); err != nil {
				return ctrl.Result{}, err
			}

			r.poller = nil

			// nothing left to do
			return ctrl.Result{}, nil
		}
	}

	if plz.IsTrue(v1alpha1.PLZRegistered) {
		if r.poller != nil && !r.poller.IsPolling() {
			r.poller.Start()
			r.startFactory(plz, r.poller.GetTestRuns())
			logger.Info("Started polling k6 Cloud for new test runs.")
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PrivateLoadZoneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k6v1alpha1.PrivateLoadZone{}).
		Complete(r)
}

// UpdateStatus is now using similar logic to TestRunReconciler:
// see if it can / should be refactored.
func (r *PrivateLoadZoneReconciler) UpdateStatus(
	ctx context.Context,
	plz *v1alpha1.PrivateLoadZone,
	log logr.Logger) (updateHappened bool, err error) {

	proposedStatus := plz.Status

	// re-fetch resource
	err = r.Get(ctx, types.NamespacedName{Namespace: plz.Namespace, Name: plz.Name}, plz)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			log.Info("Request deleted. No status to update.")
			return false, nil
		}
		log.Error(err, "Could not fetch request")
		return false, err
	}

	cleanObj := plz.DeepCopyObject().(client.Object)

	// Update only if it's truly a newer version of the resource
	// in comparison to the recently fetched resource.
	isNewer := plz.Status.SetIfNewer(proposedStatus)
	if !isNewer {
		return false, nil
	}

	err = r.Client.Status().Patch(ctx, plz, client.MergeFrom(cleanObj))

	// TODO: look into retry.RetryOnConflict(retry.DefaultRetry, func() error{...})
	// to have retries of failing update here, in case of conflicts;
	// with optional retry bool arg probably.

	// TODO: what if resource was deleted right before Patch?
	// Add a check for IsNotFound(err).

	if err != nil {
		log.Error(err, "Could not update status of custom resource")
		return false, err
	}

	return true, nil
}

func k6CloudHost() string {
	host, _ := os.LookupEnv("K6_CLOUD_HOST")
	return host
}
