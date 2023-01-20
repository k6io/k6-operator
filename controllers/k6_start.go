package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/grafana/k6-operator/api/v1alpha1"
	"github.com/grafana/k6-operator/pkg/resources/jobs"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"net/http"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

var backoffSchedule = []time.Duration{
	1 * time.Second,
	3 * time.Second,
	5 * time.Second,
}

func isServiceReady(log logr.Logger, service *v1.Service) bool {
	var resp *http.Response
	var err error
	for _, backoff := range backoffSchedule {
		resp, err = http.Get(fmt.Sprintf("http://%v.%v.svc.cluster.local:6565/v1/status", service.ObjectMeta.Name, service.ObjectMeta.Namespace))

		if err == nil && resp.StatusCode < 299 {
			break
		}

		time.Sleep(backoff)
	}

	if err != nil {
		log.Error(err, fmt.Sprintf("failed to get status from %v", service.ObjectMeta.Name))
		return false
	}

	return true
}

// StartJobs in the Ready phase using a curl container
func StartJobs(ctx context.Context, log logr.Logger, k6 *v1alpha1.K6, r *K6Reconciler) (ctrl.Result, error) {
	log.Info("Waiting for pods to get ready")

	allK6PodsAreReady, err := allK6RunnerPodsAreReadyToStart(ctx, log, k6, r)

	if !allK6PodsAreReady {
		return ctrl.Result{}, err
	}

	var hostnames []string

	sl := &v1.ServiceList{}
	selector := labels.SelectorFromSet(map[string]string{
		"app":    "k6",
		"k6_cr":  k6.Name,
		"runner": "true",
	})

	opts := &client.ListOptions{LabelSelector: selector, Namespace: k6.Namespace}

	if e := r.List(ctx, sl, opts); e != nil {
		log.Error(e, "Could not list services")
		return ctrl.Result{}, e
	}

	for _, service := range sl.Items {
		hostnames = append(hostnames, service.Spec.ClusterIP)

		if !isServiceReady(log, &service) {
			log.Info(fmt.Sprintf("%v service is not ready, aborting", service.ObjectMeta.Name))
			return ctrl.Result{}, nil
		} else {
			log.Info(fmt.Sprintf("%v service is ready", service.ObjectMeta.Name))
		}
	}

	log.Info("Changing stage of K6 status to started")
	k6.Status.Stage = "started"
	if err = r.Client.Status().Update(ctx, k6); err != nil {
		log.Error(err, "Could not update status of custom resource")
		return ctrl.Result{}, err
	}

	starter := jobs.NewStarterJob(k6, hostnames)

	if err = ctrl.SetControllerReference(k6, starter, r.Scheme); err != nil {
		log.Error(err, "Failed to set controller reference for the start job")
	}

	if err = r.Create(ctx, starter); err != nil {
		log.Error(err, "Failed to launch k6 test starter")
		return ctrl.Result{}, err
	}

	log.Info("Created started job")

	if err != nil {
		log.Error(err, "Failed to start all jobs")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func allK6RunnerPodsAreReadyToStart(ctx context.Context, log logr.Logger, k6 *v1alpha1.K6, r *K6Reconciler) (bool, error) {
	var err error

	selector := labels.SelectorFromSet(map[string]string{
		"app":    "k6",
		"k6_cr":  k6.Name,
		"runner": "true",
	})

	opts := &client.ListOptions{LabelSelector: selector, Namespace: k6.Namespace}
	pl := &v1.PodList{}
	if e := r.List(ctx, pl, opts); e != nil {
		log.Error(e, "Could not list pods")
		return false, e
	}

	var count int
	for _, pod := range pl.Items {
		if pod.Status.Phase != "Running" {
			continue
		}
		count++
	}

	log.Info(fmt.Sprintf("%d/%d runner pods ready", count, k6.Spec.Parallelism))

	if count != int(k6.Spec.Parallelism) {
		return false, nil
	}


	return true, err
}
