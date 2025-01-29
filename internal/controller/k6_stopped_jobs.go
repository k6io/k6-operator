package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/grafana/k6-operator/api/v1alpha1"
	k6api "go.k6.io/k6/api/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func isJobRunning(log logr.Logger, service *v1.Service) bool {
	resp, err := http.Get(fmt.Sprintf("http://%v:6565/v1/status", service.Spec.ClusterIP))
	if err != nil {
		return false
	}

	// Response has been received so assume the job is running.

	if resp.StatusCode >= 400 {
		log.Error(err, fmt.Sprintf("status from from runner job %v is %d", service.ObjectMeta.Name, resp.StatusCode))
		return true
	}

	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error(err, fmt.Sprintf("Error on reading status of the runner job %v", service.ObjectMeta.Name))
		return true
	}

	var status k6api.StatusJSONAPI
	if err := json.Unmarshal(data, &status); err != nil {
		log.Error(err, fmt.Sprintf("Error on parsing status of the runner job %v", service.ObjectMeta.Name))
		return true
	}

	return status.Status().Running
}

// StoppedJobs checks if the runners pods have stopped execution.
func StoppedJobs(ctx context.Context, log logr.Logger, k6 *v1alpha1.TestRun, r *TestRunReconciler) (allStopped bool) {
	if len(k6.GetStatus().TestRunID) > 0 {
		log = log.WithValues("testRunId", k6.GetStatus().TestRunID)
	}

	log.Info("Waiting for pods to stop the test run")

	selector := labels.SelectorFromSet(map[string]string{
		"app":    "k6",
		"k6_cr":  k6.NamespacedName().Name,
		"runner": "true",
	})

	opts := &client.ListOptions{LabelSelector: selector, Namespace: k6.NamespacedName().Namespace}

	sl := &v1.ServiceList{}

	if err := r.List(ctx, sl, opts); err != nil {
		log.Error(err, "Could not list services")
		return
	}

	var runningJobs int32
	for _, service := range sl.Items {

		if isJobRunning(log, &service) {
			runningJobs++
		}
	}

	log.Info(fmt.Sprintf("%d/%d runners stopped execution", k6.GetSpec().Parallelism-runningJobs, k6.GetSpec().Parallelism))

	if runningJobs > 0 {
		return
	}

	allStopped = true
	return
}

// KillJobs retrieves all runner jobs and attempts to delete them
// with propagation policy so that corresponding pods are deleted as well.
// On failure, error is returned.
// On success, error is nil and allDeleted shows if all retrieved jobs were deleted.
func KillJobs(ctx context.Context, log logr.Logger, k6 *v1alpha1.TestRun, r *TestRunReconciler) (allDeleted bool, err error) {
	if len(k6.GetStatus().TestRunID) > 0 {
		log = log.WithValues("testRunId", k6.GetStatus().TestRunID)
	}

	log.Info("Killing all runner jobs.")

	selector := labels.SelectorFromSet(map[string]string{
		"app":    "k6",
		"k6_cr":  k6.NamespacedName().Name,
		"runner": "true",
	})

	opts := &client.ListOptions{LabelSelector: selector, Namespace: k6.NamespacedName().Namespace}
	jl := &batchv1.JobList{}

	if err = r.List(ctx, jl, opts); err != nil {
		log.Error(err, "Could not list jobs")
		return
	}

	var deleteCount int

	propagationPolicy := client.PropagationPolicy(metav1.DeletionPropagation(metav1.DeletePropagationBackground))
	for _, job := range jl.Items {
		if err = r.Delete(ctx, &job, propagationPolicy); err != nil {
			log.Error(err, fmt.Sprintf("Failed to delete runner job %s", job.Name))
			// do we need to retry here?
		}
		deleteCount++
	}

	return deleteCount == len(jl.Items), nil
}
