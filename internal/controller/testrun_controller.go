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
)

const (
	k6CrLabelName = "k6_cr"
)

// TestRunReconciler reconciles a K6 object
type TestRunReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	// Note: here we assume that all users of the operator are allowed to use
	// the same token / cloud client.
	k6CloudClient *cloudapi.Client
}

// Reconcile takes a K6 object and takes the appropriate action in the cluster
// +kubebuilder:rbac:groups=k6.io,resources=testruns,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k6.io,resources=testruns/status;testruns/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods;pods/log,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;create;update
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *TestRunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("namespace", req.Namespace, "name", req.Name, "reconcileID", controller.ReconcileIDFromContext(ctx))

	// Fetch the CRD
	k6 := &v1alpha1.TestRun{}
	err := r.Get(ctx, req.NamespacedName, k6)

	if err != nil {
		if k8sErrors.IsNotFound(err) {
			log.Info("Request deleted. Nothing to reconcile.")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Could not fetch request")
		return ctrl.Result{Requeue: true}, err
	}

	return r.reconcile(ctx, req, log, k6)
}

func isCloudTestRun(k6 *v1alpha1.TestRun) bool {
	return v1alpha1.IsTrue(k6, v1alpha1.CloudTestRun) || v1alpha1.IsTrue(k6, v1alpha1.CloudPLZTestRun)
}

func (r *TestRunReconciler) reconcile(ctx context.Context, req ctrl.Request, log logr.Logger, k6 *v1alpha1.TestRun) (ctrl.Result, error) {
	var err error
	if isCloudTestRun(k6) {
		// bootstrap the client
		found, err := r.createClient(ctx, k6, log)
		if err != nil {
			log.Error(err, "A problem while getting token.")
			return ctrl.Result{}, err
		}
		if !found {
			log.Info(fmt.Sprintf("Token `%s` is not found yet.", k6.GetSpec().Token))
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
	}

	log.Info(fmt.Sprintf("Reconcile(); stage = %s", k6.GetStatus().Stage))

	// Decision making here is now a mix between stages and conditions.
	// TODO: refactor further.

	if isCloudTestRun(k6) && v1alpha1.IsFalse(k6, v1alpha1.CloudTestRunAborted) {
		// check in with the BE for status
		if r.ShouldAbort(ctx, k6, log) {
			log.Info("Received an abort signal from the k6 Cloud: stopping the test.")
			return StopJobs(ctx, log, k6, r)
		}
	}

	switch k6.GetStatus().Stage {
	case "":
		log.Info("Initialize test")

		v1alpha1.Initialize(k6)

		if _, err := r.UpdateStatus(ctx, k6, log); err != nil {
			return ctrl.Result{}, err
		}

		log.Info("Changing stage of TestRun status to initialization")
		k6.GetStatus().Stage = "initialization"

		if updateHappened, err := r.UpdateStatus(ctx, k6, log); err != nil {
			return ctrl.Result{}, err
		} else if updateHappened {
			return InitializeJobs(ctx, log, k6, r)
		}

		return ctrl.Result{}, nil

	case "initialization":
		res, ready, err := RunValidations(ctx, log, k6, r)
		if err != nil || !ready {
			if t, ok := v1alpha1.LastUpdate(k6, v1alpha1.TestRunRunning); !ok {
				// this should never happen
				return res, errors.New("Cannot find condition TestRunRunning")
			} else {
				// let's try this approach
				if time.Since(t).Minutes() > 5 {
					msg := fmt.Sprintf(errMessageTooLong, "initializer pod", "initializer job and pod")
					log.Info(msg)

					if isCloudTestRun(k6) {
						events := cloud.ErrorEvent(cloud.K6OperatorStartError).
							WithDetail(msg).
							WithAbort()
						cloud.SendTestRunEvents(r.k6CloudClient, k6.TestRunID(), log, events)
					}
				}
			}

			return res, err
		}

		if v1alpha1.IsFalse(k6, v1alpha1.CloudTestRun) {
			// RunValidations has already happened and this is not a
			// cloud test: we can move on
			log.Info("Changing stage of TestRun status to initialized")

			k6.GetStatus().Stage = "initialized"

			if updateHappened, err := r.UpdateStatus(ctx, k6, log); err != nil {
				return ctrl.Result{}, err
			} else if updateHappened {
				return ctrl.Result{}, nil
			}
		}

		if v1alpha1.IsTrue(k6, v1alpha1.CloudTestRun) {

			if v1alpha1.IsFalse(k6, v1alpha1.CloudTestRunCreated) {
				return SetupCloudTest(ctx, log, k6, r)

			} else {
				// if test run was created, then only changing status is left
				log.Info("Changing stage of TestRun status to initialized")

				k6.GetStatus().Stage = "initialized"

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
		if v1alpha1.IsTrue(k6, v1alpha1.CloudTestRun) && v1alpha1.IsTrue(k6, v1alpha1.CloudTestRunFinalized) {
			// a fluke - nothing to do
			return ctrl.Result{}, nil
		}

		if v1alpha1.IsTrue(k6, v1alpha1.CloudTestRunAborted) {
			// a fluke - nothing to do
			return ctrl.Result{}, nil
		}

		if v1alpha1.IsTrue(k6, v1alpha1.CloudPLZTestRun) {
			runningTime, _ := v1alpha1.LastUpdate(k6, v1alpha1.TestRunRunning)

			if v1alpha1.IsFalse(k6, v1alpha1.TeardownExecuted) {
				var allJobsStopped bool
				// TODO: figure out baseline time
				if time.Since(runningTime) > time.Second*30 {
					allJobsStopped = StoppedJobs(ctx, log, k6, r)
				}

				// The test run reached a regular stop in execution so execute teardown
				if v1alpha1.IsFalse(k6, v1alpha1.CloudTestRunAborted) && allJobsStopped {
					hostnames, err := r.hostnames(ctx, log, false, k6.ListOptions())
					if err != nil {
						return ctrl.Result{}, nil
					}
					runTeardown(ctx, hostnames, log)
					v1alpha1.UpdateCondition(k6, v1alpha1.TeardownExecuted, metav1.ConditionTrue)

					_, err = r.UpdateStatus(ctx, k6, log)
					return ctrl.Result{}, err
					// NOTE: we proceed here regardless whether teardown() is successful or not
				} else {
					// Test runs can take a long time and usually they aren't supposed
					// to be too quick. So check in only periodically.
					return ctrl.Result{RequeueAfter: time.Second * 15}, nil
				}
			}
		} else if !FinishJobs(ctx, log, k6, r) {
			// wait for the test to finish

			// TODO: confirm if this check is needed given the check in the beginning of reconcile
			if v1alpha1.IsTrue(k6, v1alpha1.CloudTestRun) && v1alpha1.IsFalse(k6, v1alpha1.CloudTestRunAborted) {
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

		if v1alpha1.IsTrue(k6, v1alpha1.TestRunRunning) {
			v1alpha1.UpdateCondition(k6, v1alpha1.TestRunRunning, metav1.ConditionFalse)

			log.Info("Changing stage of TestRun status to stopped")
			k6.GetStatus().Stage = "stopped"

			_, err := r.UpdateStatus(ctx, k6, log)
			if err != nil {
				return ctrl.Result{}, err
			}
			// log.Info(fmt.Sprintf("Debug updating status after finalize %v", updateHappened))
		}

		return ctrl.Result{}, nil

	case "stopped":
		if v1alpha1.IsTrue(k6, v1alpha1.CloudPLZTestRun) && v1alpha1.IsTrue(k6, v1alpha1.CloudTestRunAborted) {
			// This is a "forced" abort of the PLZ test run.
			// Wait until all the test runs are stopped, kill jobs and proceed.
			if StoppedJobs(ctx, log, k6, r) {
				if allDeleted, err := KillJobs(ctx, log, k6, r); err != nil {
					return ctrl.Result{RequeueAfter: time.Second}, err
				} else {
					// if we just have deleted all jobs, update status and go for reconcile
					if allDeleted {
						v1alpha1.UpdateCondition(k6, v1alpha1.CloudTestRunAborted, metav1.ConditionTrue)
						_, err := r.UpdateStatus(ctx, k6, log)
						if err != nil {
							return ctrl.Result{}, err
						}
					}
				}
			}
		}

		// If this is a cloud test run in any mode, try to finalize it.
		if v1alpha1.IsTrue(k6, v1alpha1.CloudTestRun) &&
			v1alpha1.IsFalse(k6, v1alpha1.CloudTestRunFinalized) {

			// If TestRunRunning has just been updated, wait for a bit before
			// acting, to avoid race condition between different reconcile loops.
			t, _ := v1alpha1.LastUpdate(k6, v1alpha1.TestRunRunning)
			if time.Since(t) < 5*time.Second {
				return ctrl.Result{RequeueAfter: time.Second * 2}, nil
			}

			if err = cloud.FinishTestRun(r.k6CloudClient, k6.GetStatus().TestRunID); err != nil {
				log.Error(err, "Failed to finalize the test run with cloud output")
				return ctrl.Result{}, nil
			} else {
				log.Info(fmt.Sprintf("Cloud test run %s was finalized successfully", k6.GetStatus().TestRunID))

				v1alpha1.UpdateCondition(k6, v1alpha1.CloudTestRunFinalized, metav1.ConditionTrue)
			}
		}

		log.Info("Changing stage of TestRun status to finished")
		k6.GetStatus().Stage = "finished"

		_, err = r.UpdateStatus(ctx, k6, log)
		if err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{RequeueAfter: time.Second}, nil

	case "error", "finished":
		// delete if configured
		if k6.GetSpec().Cleanup == "post" {
			log.Info("Cleaning up all resources")
			_ = r.Delete(ctx, k6)
		}
		// notify if configured
		return ctrl.Result{}, nil
	}

	err = fmt.Errorf("invalid status")
	log.Error(err, "Invalid status for the k6 resource.")
	return ctrl.Result{}, err
}

// SetupWithManager sets up a managed controller that will reconcile all events for the K6 CRD
func (r *TestRunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.TestRun{}).
		Owns(&batchv1.Job{}).
		Watches(&v1.Pod{},
			handler.EnqueueRequestsFromMapFunc(
				func(ctx context.Context, object client.Object) []reconcile.Request {
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
					return ok
				}))).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
			// RateLimiter - ?
		}).
		Complete(r)
}

func (r *TestRunReconciler) UpdateStatus(ctx context.Context, k6 *v1alpha1.TestRun, log logr.Logger) (updateHappened bool, err error) {
	proposedStatus := k6.GetStatus().DeepCopy()

	// re-fetch resource
	err = r.Get(ctx, k6.NamespacedName(), k6)
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
	isNewer := k6.GetStatus().SetIfNewer(*proposedStatus)
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
func (r *TestRunReconciler) ShouldAbort(ctx context.Context, k6 *v1alpha1.TestRun, log logr.Logger) bool {
	// sanity check
	if len(k6.TestRunID()) == 0 {
		// log.Error(errors.New("empty test run ID"), "Trying to get state of test run with empty test run ID")
		return false
	}

	status, err := cloud.GetTestRunState(r.k6CloudClient, k6.TestRunID(), log)
	if err != nil {
		log.Error(err, "Failed to get test run state.")
		return false
	}

	isAborted := status.Aborted()

	log.Info(fmt.Sprintf("Received test run status %v", status))

	return isAborted
}

func (r *TestRunReconciler) createClient(ctx context.Context, k6 *v1alpha1.TestRun, log logr.Logger) (bool, error) {
	if r.k6CloudClient == nil {
		token, tokenReady, err := loadToken(ctx, log, r.Client, k6.GetSpec().Token, &client.ListOptions{Namespace: k6.NamespacedName().Namespace})
		if err != nil {
			log.Error(err, "A problem while getting token.")
			return false, err
		}
		if !tokenReady {
			return false, nil
		}

		host := getEnvVar(k6.GetSpec().Runner.Env, "K6_CLOUD_HOST")

		r.k6CloudClient = cloud.NewClient(log, token, host)
	}

	return true, nil
}
