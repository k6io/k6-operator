package jobs

import (
	"fmt"
	"strconv"

	"github.com/grafana/k6-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewInitializerJob builds a template used to initializefor creating a starter job
func NewInitializerJob(k6 *v1alpha1.TestRun, argLine string) (*batchv1.Job, error) {
	script, err := k6.GetSpec().ParseScript()
	if err != nil {
		return nil, err
	}

	var (
		image                        = "ghcr.io/grafana/k6-operator:latest-runner"
		annotations                  = make(map[string]string)
		labels                       = newLabels(k6.NamespacedName().Name)
		serviceAccountName           = "default"
		automountServiceAccountToken = true
		ports                        = append([]corev1.ContainerPort{{ContainerPort: 6565}}, k6.GetSpec().Ports...)
	)

	if k6.GetSpec().Initializer == nil {
		k6.GetSpec().Initializer = k6.GetSpec().Runner.DeepCopy()
	}

	if k6.GetSpec().Initializer.Image != "" {
		image = k6.GetSpec().Initializer.Image
	}

	if k6.GetSpec().Initializer.Metadata.Annotations != nil {
		annotations = k6.GetSpec().Initializer.Metadata.Annotations
	}

	if k6.GetSpec().Initializer.Metadata.Labels != nil {
		for k, v := range k6.GetSpec().Initializer.Metadata.Labels {
			if _, ok := labels[k]; !ok {
				labels[k] = v
			}
		}
	}

	if k6.GetSpec().Initializer.ServiceAccountName != "" {
		serviceAccountName = k6.GetSpec().Initializer.ServiceAccountName
	}

	if k6.GetSpec().Initializer.AutomountServiceAccountToken != "" {
		automountServiceAccountToken, _ = strconv.ParseBool(k6.GetSpec().Initializer.AutomountServiceAccountToken)
	}

	var (
		// k6 allows to run archive command on archives too so type of file here doesn't matter
		scriptName  = script.FullName()
		archiveName = fmt.Sprintf("/tmp/%s.archived.tar", script.Filename)
	)
	istioCommand, istioEnabled := newIstioCommand(k6.GetSpec().Scuttle.Enabled, []string{"sh", "-c"})
	command := append(istioCommand, fmt.Sprintf(
		// There can be several scenarios from k6 command here:
		// a) script is correct and `k6 inspect` outputs JSON
		// b) script is partially incorrect and `k6` outputs a warning log message and
		// then a JSON
		// c) script is incorrect and `k6` outputs an error log message
		// Warnings at this point are not necessary (warning messages will re-appear in
		// runner's logs and the user can see them there) so we need a pure JSON here
		// without any additional messages in cases a) and b). In case c), output should
		// contain error message and the Job is to exit with non-zero code.
		//
		// Due to some pecularities of k6 logging, to achieve the above behaviour,
		// we need to use a workaround to store all log messages in temp file while
		// printing JSON as usual. Then parse temp file only for errors, ignoring
		// any other log messages.
		// Related: https://github.com/grafana/k6-docs/issues/877
		"mkdir -p $(dirname %s) && k6 archive %s -O %s %s 2> /tmp/k6logs && k6 inspect --execution-requirements %s 2> /tmp/k6logs && ! cat /tmp/k6logs | grep 'level=error'",
		archiveName, scriptName, archiveName, argLine,
		archiveName))

	env := append(newIstioEnvVar(k6.GetSpec().Scuttle, istioEnabled), k6.GetSpec().Initializer.Env...)

	volumes := script.Volume()
	volumes = append(volumes, k6.GetSpec().Initializer.Volumes...)

	volumeMounts := script.VolumeMount()
	volumeMounts = append(volumeMounts, k6.GetSpec().Initializer.VolumeMounts...)

	var zero32 int32
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-initializer", k6.NamespacedName().Name),
			Namespace:   k6.NamespacedName().Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &zero32,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken: &automountServiceAccountToken,
					ServiceAccountName:           serviceAccountName,
					Affinity:                     k6.GetSpec().Initializer.Affinity,
					NodeSelector:                 k6.GetSpec().Initializer.NodeSelector,
					Tolerations:                  k6.GetSpec().Initializer.Tolerations,
					TopologySpreadConstraints:    k6.GetSpec().Initializer.TopologySpreadConstraints,
					SecurityContext:              &k6.GetSpec().Initializer.SecurityContext,
					RestartPolicy:                corev1.RestartPolicyNever,
					ImagePullSecrets:             k6.GetSpec().Initializer.ImagePullSecrets,
					InitContainers:               getInitContainers(k6.GetSpec().Initializer, script),
					Containers: []corev1.Container{
						{
							Image:           image,
							ImagePullPolicy: k6.GetSpec().Initializer.ImagePullPolicy,
							Name:            "k6",
							Command:         command,
							Env:             env,
							Resources:       k6.GetSpec().Initializer.Resources,
							VolumeMounts:    volumeMounts,
							EnvFrom:         k6.GetSpec().Initializer.EnvFrom,
							Ports:           ports,
							SecurityContext: &k6.GetSpec().Initializer.ContainerSecurityContext,
						},
					},
					Volumes: volumes,
				},
			},
		},
	}

	return job, nil
}
