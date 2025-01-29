package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/grafana/k6-operator/api/v1alpha1"
	"github.com/grafana/k6-operator/pkg/cloud"
	"github.com/grafana/k6-operator/pkg/resources/jobs"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateJobs creates jobs that will spawn k6 pods for distributed test
func CreateJobs(ctx context.Context, log logr.Logger, k6 *v1alpha1.TestRun, r *TestRunReconciler) (ctrl.Result, error) {
	var (
		err   error
		token string // only for cloud tests
	)

	if v1alpha1.IsTrue(k6, v1alpha1.CloudTestRun) && v1alpha1.IsTrue(k6, v1alpha1.CloudTestRunCreated) {
		log = log.WithValues("testRunId", k6.GetStatus().TestRunID)

		var (
			tokenReady bool
			sOpts      *client.ListOptions
		)

		if v1alpha1.IsTrue(k6, v1alpha1.CloudPLZTestRun) {
			sOpts = &client.ListOptions{Namespace: k6.NamespacedName().Namespace}
		}

		token, tokenReady, err = loadToken(ctx, log, r.Client, k6.GetSpec().Token, sOpts)
		if err != nil {
			// An error here means a very likely mis-configuration of the token.
			// TODO: update status to error to let a user know quicker
			log.Error(err, "A problem while getting token.")
			return ctrl.Result{}, nil
		}
		if !tokenReady {
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}
	}

	log.Info("Creating test jobs")

	if res, recheck, err := createJobSpecs(ctx, log, k6, r, token); err != nil {
		if v1alpha1.IsTrue(k6, v1alpha1.CloudTestRun) {
			events := cloud.ErrorEvent(cloud.K6OperatorStartError).
				WithDetail(fmt.Sprintf("Failed to create runner jobs: %v", err)).
				WithAbort()
			cloud.SendTestRunEvents(r.k6CloudClient, k6.TestRunID(), log, events)
		}

		return res, err
	} else if recheck {
		return res, nil
	}

	log.Info("Changing stage of TestRun status to created")
	k6.GetStatus().Stage = "created"

	if updateHappened, err := r.UpdateStatus(ctx, k6, log); err != nil {
		return ctrl.Result{}, err
	} else if updateHappened {
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{}, nil
}

func createJobSpecs(ctx context.Context, log logr.Logger, k6 *v1alpha1.TestRun, r *TestRunReconciler, token string) (ctrl.Result, bool, error) {
	found := &batchv1.Job{}
	namespacedName := types.NamespacedName{
		Name:      fmt.Sprintf("%s-1", k6.NamespacedName().Name),
		Namespace: k6.NamespacedName().Namespace,
	}

	if err := r.Get(ctx, namespacedName, found); err == nil || !errors.IsNotFound(err) {
		if err == nil {
			err = fmt.Errorf("job with the name %s exists; make sure you've deleted your previous run", namespacedName.Name)
		}
		log.Info(err.Error())

		// is it possible to implement this delay with resourceVersion of the job?

		t, condUpdated := v1alpha1.LastUpdate(k6, v1alpha1.CloudTestRun)
		// If condition is unknown then resource hasn't been updated with `k6 inspect` results.
		// If it has been updated but very recently, wait a bit before throwing an error.
		if v1alpha1.IsUnknown(k6, v1alpha1.CloudTestRun) || !condUpdated || time.Since(t).Seconds() <= 30 {
			// try again before returning an error
			return ctrl.Result{RequeueAfter: time.Second * 10}, true, nil
		}

		return ctrl.Result{}, false, err
	}

	for i := 1; i <= int(k6.GetSpec().Parallelism); i++ {
		if err := launchTest(ctx, k6, i, log, r, token); err != nil {
			return ctrl.Result{}, false, err
		}
	}
	return ctrl.Result{}, false, nil
}

func launchTest(ctx context.Context, k6 *v1alpha1.TestRun, index int, log logr.Logger, r *TestRunReconciler, token string) error {
	var job *batchv1.Job
	var service *corev1.Service
	var err error

	msg := fmt.Sprintf("Launching k6 test #%d", index)
	log.Info(msg)

	if job, err = jobs.NewRunnerJob(k6, index, token); err != nil {
		log.Error(err, "Failed to generate k6 test job")
		return err
	}

	log.Info(fmt.Sprintf("Runner job is ready to start with image `%s` and command `%s`",
		job.Spec.Template.Spec.Containers[0].Image, job.Spec.Template.Spec.Containers[0].Command))

	if err = ctrl.SetControllerReference(k6, job, r.Scheme); err != nil {
		log.Error(err, "Failed to set controller reference for job")
		return err
	}

	if err = r.Create(ctx, job); err != nil {
		log.Error(err, "Failed to launch k6 test")
		return err
	}

	if service, err = jobs.NewRunnerService(k6, index); err != nil {
		log.Error(err, "Failed to generate k6 test service")
		return err
	}

	if err = ctrl.SetControllerReference(k6, service, r.Scheme); err != nil {
		log.Error(err, "Failed to set controller reference for service")
		return err
	}

	if err = r.Create(ctx, service); err != nil {
		log.Error(err, "Failed to launch k6 test services")
		return err
	}

	return nil
}
