package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/grafana/k6-operator/api/v1alpha1"
	"github.com/grafana/k6-operator/pkg/cloud"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FinishJobs checks if the runners pods have finished execution.
func FinishJobs(ctx context.Context, log logr.Logger, k6 *v1alpha1.TestRun, r *TestRunReconciler) (allFinished bool) {
	if len(k6.GetStatus().TestRunID) > 0 {
		log = log.WithValues("testRunId", k6.GetStatus().TestRunID)
	}

	log.Info("Checking if all runner pods are finished")

	selector := labels.SelectorFromSet(map[string]string{
		"app":    "k6",
		"k6_cr":  k6.NamespacedName().Name,
		"runner": "true",
	})

	opts := &client.ListOptions{LabelSelector: selector, Namespace: k6.NamespacedName().Namespace}
	jl := &batchv1.JobList{}
	var err error

	if err = r.List(ctx, jl, opts); err != nil {
		log.Error(err, "Could not list jobs")
		return
	}

	// TODO: We should distinguish between Suceeded/Failed/Unknown
	var (
		finished, failed int32
	)
	for _, job := range jl.Items {
		if job.Status.Active != 0 {
			continue
		}
		finished++

		if job.Status.Failed > 0 {
			failed++
		}
	}

	msg := fmt.Sprintf("%d/%d jobs complete, %d failed", finished, k6.GetSpec().Parallelism, failed)
	log.Info(msg)

	if v1alpha1.IsTrue(k6, v1alpha1.CloudTestRun) && failed > 0 {
		events := cloud.ErrorEvent(cloud.K6OperatorRunnerError).
			WithDetail(msg).
			WithAbort()
		cloud.SendTestRunEvents(r.k6CloudClient, k6.TestRunID(), log, events)
	}

	if finished < k6.GetSpec().Parallelism {
		return
	}

	allFinished = true
	return
}
