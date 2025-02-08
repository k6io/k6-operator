package jobs

import (
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/util/intstr"

	"strings"

	"github.com/grafana/k6-operator/api/v1alpha1"
	"github.com/grafana/k6-operator/pkg/cloud"
	"github.com/grafana/k6-operator/pkg/segmentation"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewRunnerJob creates a new k6 job from a CRD
func NewRunnerJob(k6 *v1alpha1.TestRun, index int, token string) (*batchv1.Job, error) {
	name := fmt.Sprintf("%s-%d", k6.NamespacedName().Name, index)
	postCommand := []string{"k6", "run"}

	command, istioEnabled := newIstioCommand(k6.GetSpec().Scuttle.Enabled, postCommand)

	quiet := true
	if k6.GetSpec().Quiet != "" {
		quiet, _ = strconv.ParseBool(k6.GetSpec().Quiet)
	}

	if quiet {
		command = append(command, "--quiet")
	}

	if k6.GetSpec().Parallelism > 1 {
		var args []string
		var err error

		if args, err = segmentation.NewCommandFragments(index, int(k6.GetSpec().Parallelism)); err != nil {
			return nil, err

		}
		command = append(command, args...)
	}

	script, err := k6.GetSpec().ParseScript()
	if err != nil {
		return nil, err
	}

	if k6.GetSpec().Arguments != "" {
		args := strings.Split(k6.GetSpec().Arguments, " ")
		command = append(command, args...)
	}

	command = append(
		command,
		script.FullName(),
		"--address=0.0.0.0:6565")

	paused := true
	if k6.GetSpec().Paused != "" {
		paused, _ = strconv.ParseBool(k6.GetSpec().Paused)
	}

	if paused {
		command = append(command, "--paused")
	}

	// Add an instance tag: in case metrics are stored, they need to be distinguished by instance
	command = append(command, "--tag", fmt.Sprintf("instance_id=%d", index))

	// Add an job tag: in case metrics are stored, they need to be distinguished by job
	command = append(command, "--tag", fmt.Sprintf("job_name=%s", name))

	if v1alpha1.IsTrue(k6, v1alpha1.CloudPLZTestRun) {
		command = append(command, "--no-setup", "--no-teardown", "--linger")
	}

	command = script.UpdateCommand(command)

	var (
		zero   int64 = 0
		zero32 int32 = 0
	)

	image := "ghcr.io/grafana/k6-operator:latest-runner"
	if k6.GetSpec().Runner.Image != "" {
		image = k6.GetSpec().Runner.Image
	}

	runnerAnnotations := make(map[string]string)
	if k6.GetSpec().Runner.Metadata.Annotations != nil {
		runnerAnnotations = k6.GetSpec().Runner.Metadata.Annotations
	}

	runnerLabels := newLabels(k6.NamespacedName().Name)
	runnerLabels["runner"] = "true"
	if k6.GetSpec().Runner.Metadata.Labels != nil {
		for k, v := range k6.GetSpec().Runner.Metadata.Labels { // Order not specified
			if _, ok := runnerLabels[k]; !ok {
				runnerLabels[k] = v
			}
		}
	}

	serviceAccountName := "default"
	if k6.GetSpec().Runner.ServiceAccountName != "" {
		serviceAccountName = k6.GetSpec().Runner.ServiceAccountName
	}

	automountServiceAccountToken := true
	if k6.GetSpec().Runner.AutomountServiceAccountToken != "" {
		automountServiceAccountToken, _ = strconv.ParseBool(k6.GetSpec().Runner.AutomountServiceAccountToken)
	}

	ports := []corev1.ContainerPort{{ContainerPort: 6565}}
	ports = append(ports, k6.GetSpec().Ports...)

	env := newIstioEnvVar(k6.GetSpec().Scuttle, istioEnabled)

	// this is a cloud output run
	if len(k6.GetStatus().TestRunID) > 0 {
		// temporary hack
		if v1alpha1.IsTrue(k6, v1alpha1.CloudPLZTestRun) {
			k6.GetStatus().AggregationVars = "2|5s|3s|10s|10"
		}

		aggregationVars, err := cloud.DecodeAggregationConfig(k6.GetStatus().AggregationVars)
		if err != nil {
			return nil, err
		}
		env = append(env, aggregationVars...)
		env = append(env, corev1.EnvVar{
			Name:  "K6_CLOUD_PUSH_REF_ID",
			Value: k6.GetStatus().TestRunID,
		}, corev1.EnvVar{
			Name:  "K6_CLOUD_TOKEN",
			Value: token,
		},
		)
	}

	env = append(env, k6.GetSpec().Runner.Env...)

	volumes := script.Volume()
	volumes = append(volumes, k6.GetSpec().Runner.Volumes...)

	volumeMounts := script.VolumeMount()
	volumeMounts = append(volumeMounts, k6.GetSpec().Runner.VolumeMounts...)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   k6.NamespacedName().Namespace,
			Labels:      runnerLabels,
			Annotations: runnerAnnotations,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &zero32,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      runnerLabels,
					Annotations: runnerAnnotations,
				},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken: &automountServiceAccountToken,
					ServiceAccountName:           serviceAccountName,
					Hostname:                     name,
					RestartPolicy:                corev1.RestartPolicyNever,
					Affinity:                     k6.GetSpec().Runner.Affinity,
					NodeSelector:                 k6.GetSpec().Runner.NodeSelector,
					Tolerations:                  k6.GetSpec().Runner.Tolerations,
					TopologySpreadConstraints:    k6.GetSpec().Runner.TopologySpreadConstraints,
					SecurityContext:              &k6.GetSpec().Runner.SecurityContext,
					ImagePullSecrets:             k6.GetSpec().Runner.ImagePullSecrets,
					InitContainers:               getInitContainers(&k6.GetSpec().Runner, script),
					Containers: []corev1.Container{{
						Image:           image,
						ImagePullPolicy: k6.GetSpec().Runner.ImagePullPolicy,
						Name:            "k6",
						Command:         command,
						Env:             env,
						Resources:       k6.GetSpec().Runner.Resources,
						VolumeMounts:    volumeMounts,
						Ports:           ports,
						EnvFrom:         k6.GetSpec().Runner.EnvFrom,
						LivenessProbe:   generateLivenessProbe(k6.GetSpec().Runner.LivenessProbe),
						ReadinessProbe:  generateReadinessProbe(k6.GetSpec().Runner.ReadinessProbe),
						SecurityContext: &k6.GetSpec().Runner.ContainerSecurityContext,
					}},
					TerminationGracePeriodSeconds: &zero,
					Volumes:                       volumes,
				},
			},
		},
	}

	if k6.GetSpec().Separate {
		job.Spec.Template.Spec.Affinity = newAntiAffinity()
	}

	return job, nil
}

func NewRunnerService(k6 *v1alpha1.TestRun, index int) (*corev1.Service, error) {
	serviceName := fmt.Sprintf("%s-%s-%d", k6.NamespacedName().Name, "service", index)
	runnerName := fmt.Sprintf("%s-%d", k6.NamespacedName().Name, index)

	runnerAnnotations := make(map[string]string)
	if k6.GetSpec().Runner.Metadata.Annotations != nil {
		runnerAnnotations = k6.GetSpec().Runner.Metadata.Annotations
	}

	runnerLabels := newLabels(k6.NamespacedName().Name)
	runnerLabels["runner"] = "true"
	if k6.GetSpec().Runner.Metadata.Labels != nil {
		for k, v := range k6.GetSpec().Runner.Metadata.Labels { // Order not specified
			if _, ok := runnerLabels[k]; !ok {
				runnerLabels[k] = v
			}
		}
	}

	port := []corev1.ServicePort{{
		Name:     "http-api",
		Port:     6565,
		Protocol: "TCP",
	}}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        serviceName,
			Namespace:   k6.NamespacedName().Namespace,
			Labels:      runnerLabels,
			Annotations: runnerAnnotations,
		},
		Spec: corev1.ServiceSpec{
			Ports: port,
			Selector: map[string]string{
				"job-name": runnerName,
			},
		},
	}

	return service, nil
}

func newAntiAffinity() *corev1.Affinity {
	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "app",
								Operator: "In",
								Values: []string{
									"k6",
								},
							},
							{
								Key:      "runner",
								Operator: "In",
								Values: []string{
									"true",
								},
							},
						},
					},
					TopologyKey: "kubernetes.io/hostname",
				},
			},
		},
	}
}

func generateLivenessProbe(configuredProbe *corev1.Probe) *corev1.Probe {
	if configuredProbe != nil {
		return configuredProbe
	}
	return &corev1.Probe{
		InitialDelaySeconds: 10,
		TimeoutSeconds:      3,
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/v1/status",
				Port:   intstr.IntOrString{IntVal: 6565},
				Scheme: "HTTP",
			},
		},
	}
}

func generateReadinessProbe(configuredProbe *corev1.Probe) *corev1.Probe {
	if configuredProbe != nil {
		return configuredProbe
	}
	return &corev1.Probe{
		TimeoutSeconds: 3,
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/v1/status",
				Port:   intstr.IntOrString{IntVal: 6565},
				Scheme: "HTTP",
			},
		},
	}
}
