package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/grafana/k6-operator/api/v1alpha1"
	"github.com/grafana/k6-operator/pkg/resources/jobs"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// StopJobs in the Ready phase using a curl container
// It assumes that Services of the runners are already up and
// test is being executed.
func StopJobs(ctx context.Context, log logr.Logger, k6 *v1alpha1.TestRun, r *TestRunReconciler) (res ctrl.Result, err error) {
	if len(k6.GetStatus().TestRunID) > 0 {
		log = log.WithValues("testRunId", k6.GetStatus().TestRunID)
	}

	selector := labels.SelectorFromSet(map[string]string{
		"app":    "k6",
		"k6_cr":  k6.NamespacedName().Name,
		"runner": "true",
	})

	opts := &client.ListOptions{LabelSelector: selector, Namespace: k6.NamespacedName().Namespace}

	var hostnames []string
	sl := &v1.ServiceList{}

	if err = r.List(ctx, sl, opts); err != nil {
		log.Error(err, "Could not list services")
		return res, nil
	}

	for _, service := range sl.Items {
		hostnames = append(hostnames, service.Spec.ClusterIP)
	}

	stopJob := jobs.NewStopJob(k6, hostnames)

	if err = ctrl.SetControllerReference(k6, stopJob, r.Scheme); err != nil {
		log.Error(err, "Failed to set controller reference for the stop job")
	}

	// TODO: add a check for existence of stop job

	if err = r.Create(ctx, stopJob); err != nil {
		log.Error(err, "Failed to launch k6 test stop job.")
		return res, nil
	}

	log.Info("Created stop job")

	log.Info("Changing stage of TestRun status to stopped")
	k6.GetStatus().Stage = "stopped"
	v1alpha1.UpdateCondition(k6, v1alpha1.TestRunRunning, metav1.ConditionFalse)
	v1alpha1.UpdateCondition(k6, v1alpha1.CloudTestRunAborted, metav1.ConditionTrue)

	if updateHappened, err := r.UpdateStatus(ctx, k6, log); err != nil {
		return ctrl.Result{}, err
	} else if updateHappened {
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}
	return ctrl.Result{}, nil
}
