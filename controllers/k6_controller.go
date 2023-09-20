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
	"errors"
	"fmt"
	"time"

	"go.k6.io/k6/cloudapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/go-logr/logr"
	"github.com/grafana/k6-operator/api/v1alpha1"
	"github.com/grafana/k6-operator/pkg/cloud"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const k6CrLabelName = "k6_cr"

// K6Reconciler reconciles a K6 object
type K6Reconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	// Note: here we assume that all users of the operator are allowed to use
	// the same token / cloud client.
	k6CloudClient *cloudapi.Client
}

// Reconcile takes a K6 object and takes the appropriate action in the cluster
// +kubebuilder:rbac:groups=k6.io,resources=k6s,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k6.io,resources=k6s/status;k6s/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods;pods/log,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;create;update
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *K6Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("namespace", req.Namespace, "name", req.Name, "reconcileID", controller.ReconcileIDFromContext(ctx))

	// Fetch the CRD
	k6 := &v1alpha1.K6{}
	err := r.Get(ctx, req.NamespacedName, k6)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			log.Info("Request deleted. Nothing to reconcile.")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Could not fetch request")
		return ctrl.Result{Requeue: true}, err
	}

	if k6.IsTrue(v1alpha1.CloudPLZTestRun) {
		// bootstrap the client
		found, err := r.createClient(ctx, k6, log)
		if err != nil {
			log.Error(err, "A problem while getting token.")
			return ctrl.Result{}, err
		}
		if !found {
			log.Info(fmt.Sprintf("Token `%s` is not found yet.", k6.Spec.Token))
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
	}

	log.Info(fmt.Sprintf("Reconcile(); stage = %s", k6.Status.Stage))

	// Decision making here is now a mix between stages and conditions.
	// TODO: refactor further.

	switch k6.Status.Stage {
	case "":
		log.Info("Initialize test")

		k6.Initialize()

		if _, err := r.UpdateStatus(ctx, k6, log); err != nil {
			return ctrl.Result{}, err
		}

		log.Info("Changing stage of K6 status to initialization")
		k6.Status.Stage = "initialization"
		if updateHappened, err := r.UpdateStatus(ctx, k6, log); err != nil {
			return ctrl.Result{}, err
		} else if updateHappened {
			return InitializeJobs(ctx, log, k6, r)
		}
		return ctrl.Result{}, nil

	case "initialization":
		if k6.IsUnknown(v1alpha1.CloudTestRun) {
			return RunValidations(ctx, log, k6, r)
		}

		if k6.IsFalse(v1alpha1.CloudTestRun) {
			// RunValidations has already happened and this is not a
			// cloud test: we can move on
			log.Info("Changing stage of K6 status to initialized")

			k6.Status.Stage = "initialized"

			if updateHappened, err := r.UpdateStatus(ctx, k6, log); err != nil {
				return ctrl.Result{}, err
			} else if updateHappened {
				return ctrl.Result{}, nil
			}
		}

		if k6.IsTrue(v1alpha1.CloudTestRun) {

			if k6.IsFalse(v1alpha1.CloudTestRunCreated) {
				return SetupCloudTest(ctx, log, k6, r)

			} else {
				// if test run was created, then only changing status is left
				log.Info("Changing stage of K6 status to initialized")

				k6.Status.Stage = "initialized"

				if _, err := r.UpdateStatus(ctx, k6, log); err != nil {
					return ctrl.Result{}, err
				}
			}
		}

		return ctrl.Result{}, nil

	case "initialized":
		return CreateJobs(ctx, log, k6, r)

	case "created":
		return StartJobs(ctx, log, k6, r)

	case "started":
		if k6.IsTrue(v1alpha1.CloudTestRun) && k6.IsTrue(v1alpha1.CloudTestRunFinalized) {
			// a fluke - nothing to do
			return ctrl.Result{}, nil
		}

		if k6.IsTrue(v1alpha1.CloudTestRunAborted) {
			// a fluke - nothing to do
			return ctrl.Result{}, nil
		}

		// wait for the test to finish
		if !FinishJobs(ctx, log, k6, r) {

			if k6.IsTrue(v1alpha1.CloudPLZTestRun) && k6.IsFalse(v1alpha1.CloudTestRunAborted) {
				// check in with the BE for status
				if r.ShouldAbort(ctx, k6, log) {
					log.Info("Received an abort signal from the k6 Cloud: stopping the test.")
					return StopJobs(ctx, log, k6, r)
				}
			}

			// The test continues to execute.

			// Test runs can take a long time and usually they aren't supposed
			// to be too quick. So check in only periodically.
			return ctrl.Result{RequeueAfter: time.Second * 15}, nil
		}

		log.Info("All runner pods are finished")

		// now mark it as stopped

		if k6.IsTrue(v1alpha1.TestRunRunning) {
			k6.UpdateCondition(v1alpha1.TestRunRunning, metav1.ConditionFalse)

			log.Info("Changing stage of K6 status to stopped")
			k6.Status.Stage = "stopped"

			_, err := r.UpdateStatus(ctx, k6, log)
			if err != nil {
				return ctrl.Result{}, err
			}
			// log.Info(fmt.Sprintf("Debug updating status after finalize %v", updateHappened))
		}

		return ctrl.Result{}, nil

	case "stopped":
		if k6.IsTrue(v1alpha1.CloudPLZTestRun) && k6.IsTrue(v1alpha1.CloudTestRunAborted) {
			// This is a "forced" abort of the PLZ test run.
			// Wait until all the test runs are stopped, kill jobs and proceed.
			if StoppedJobs(ctx, log, k6, r) {
				if allDeleted, err := KillJobs(ctx, log, k6, r); err != nil {
					return ctrl.Result{RequeueAfter: time.Second}, err
				} else {
					// if we just have deleted all jobs, update status and go for reconcile
					if allDeleted {
						k6.UpdateCondition(v1alpha1.CloudTestRunAborted, metav1.ConditionTrue)
						_, err := r.UpdateStatus(ctx, k6, log)
						if err != nil {
							return ctrl.Result{}, err
						}
					}
				}
			}
		}

		// If this is a cloud test run in any mode, try to finalize it.
		if k6.IsTrue(v1alpha1.CloudTestRun) &&
			k6.IsFalse(v1alpha1.CloudTestRunFinalized) {

			// If TestRunRunning has just been updated, wait for a bit before
			// acting, to avoid race condition between different reconcile loops.
			t, _ := k6.LastUpdate(v1alpha1.TestRunRunning)
			if time.Now().Sub(t) < 5*time.Second {
				return ctrl.Result{RequeueAfter: time.Second * 2}, nil
			}

			if err = cloud.FinishTestRun(r.k6CloudClient, k6.Status.TestRunID); err != nil {
				log.Error(err, "Failed to finalize the test run with cloud output")
				return ctrl.Result{}, nil
			} else {
				log.Info(fmt.Sprintf("Cloud test run %s was finalized successfully", k6.Status.TestRunID))

				k6.UpdateCondition(v1alpha1.CloudTestRunFinalized, metav1.ConditionTrue)
			}
		}

		log.Info("Changing stage of K6 status to finished")
		k6.Status.Stage = "finished"

		_, err := r.UpdateStatus(ctx, k6, log)
		if err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{RequeueAfter: time.Second}, nil

	case "error", "finished":
		// delete if configured
		if (k6.Spec.Cleanup == "post") ||
			(k6.Status.Stage == "error" && k6.Spec.Cleanup == "whenFailed") ||
			(k6.Status.Stage == "finished" && k6.Spec.Cleanup == "whenOk") {

			log.Info("Cleaning up all resources")
			r.Delete(ctx, k6)
		}
		// notify if configured
		return ctrl.Result{}, nil
	}

	err = fmt.Errorf("invalid status")
	log.Error(err, "Invalid status for the k6 resource.")
	return ctrl.Result{}, err
}

// SetupWithManager sets up a managed controller that will reconcile all events for the K6 CRD
func (r *K6Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.K6{}).
		Owns(&batchv1.Job{}).
		Watches(&source.Kind{Type: &v1.Pod{}},
			handler.EnqueueRequestsFromMapFunc(
				func(object client.Object) []reconcile.Request {
					pod := object.(*v1.Pod)
					k6CrName, ok := pod.GetLabels()[k6CrLabelName]
					if !ok {
						return nil
					}
					return []reconcile.Request{
						{NamespacedName: types.NamespacedName{
							Name:      k6CrName,
							Namespace: object.GetNamespace(),
						}}}
				}),
			builder.WithPredicates(predicate.NewPredicateFuncs(
				func(object client.Object) bool {
					pod := object.(*v1.Pod)
					_, ok := pod.GetLabels()[k6CrLabelName]
					if !ok {
						return false
					}
					return true
				}))).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
			// RateLimiter - ?
		}).
		Complete(r)
}

func (r *K6Reconciler) UpdateStatus(ctx context.Context, k6 *v1alpha1.K6, log logr.Logger) (updateHappened bool, err error) {
	proposedStatus := k6.Status

	// re-fetch resource
	err = r.Get(ctx, types.NamespacedName{Namespace: k6.Namespace, Name: k6.Name}, k6)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			log.Info("Request deleted. No status to update.")
			return false, nil
		}
		log.Error(err, "Could not fetch request")
		return false, err
	}

	cleanObj := k6.DeepCopyObject().(client.Object)

	// Update only if it's truly a newer version of the resource
	// in comparison to the recently fetched resource.
	isNewer := k6.Status.SetIfNewer(proposedStatus)
	if !isNewer {
		return false, nil
	}

	err = r.Client.Status().Patch(ctx, k6, client.MergeFrom(cleanObj))

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

// ShouldAbort retrieves the status of test run from the Cloud and whether it should
// cause a forced stop. It is meant to be used only by PLZ test runs.
func (r *K6Reconciler) ShouldAbort(ctx context.Context, k6 *v1alpha1.K6, log logr.Logger) bool {
	// sanity check
	if len(k6.Status.TestRunID) == 0 {
		log.Error(errors.New("empty test run ID"), "Trying to get state of test run with empty test run ID")
		return false
	}

	status, err := cloud.GetTestRunState(r.k6CloudClient, k6.Status.TestRunID, log)
	if err != nil {
		log.Error(err, "Failed to get test run state.")
		return false
	}

	isAborted := status.Aborted()

	log.Info(fmt.Sprintf("Received test run status %v", status))

	return isAborted
}

func (r *K6Reconciler) createClient(ctx context.Context, k6 *v1alpha1.K6, log logr.Logger) (bool, error) {
	if r.k6CloudClient == nil {
		token, tokenReady, err := loadToken(ctx, log, r.Client, k6.Spec.Token, &client.ListOptions{Namespace: k6.Namespace})
		if err != nil {
			log.Error(err, "A problem while getting token.")
			return false, err
		}
		if !tokenReady {
			return false, nil
		}

		host := getEnvVar(k6.Spec.Runner.Env, "K6_CLOUD_HOST")

		r.k6CloudClient = cloud.NewClient(log, token, host)
	}

	return true, nil
}
