package jobs

import (
	"testing"

	deep "github.com/go-test/deep"
	"github.com/grafana/k6-operator/api/v1alpha1"
	"github.com/grafana/k6-operator/pkg/types"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewInitializerJob(t *testing.T) {
	script := &types.Script{
		Name:     "test",
		Filename: "test.js",
		Type:     "ConfigMap",
	}

	automountServiceAccountToken := true
	zero := int32(0)

	expectedOutcome := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-initializer",
			Namespace: "test",
			Labels: map[string]string{
				"app":    "k6",
				"k6_cr":  "test",
				"label1": "awesome",
			},
			Annotations: map[string]string{
				"awesomeAnnotation": "dope",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &zero,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":    "k6",
						"k6_cr":  "test",
						"label1": "awesome",
					},
					Annotations: map[string]string{
						"awesomeAnnotation": "dope",
					},
				},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken: &automountServiceAccountToken,
					ServiceAccountName:           "default",
					Affinity:                     nil,
					NodeSelector:                 nil,
					Tolerations:                  nil,
					TopologySpreadConstraints:    nil,
					RestartPolicy:                corev1.RestartPolicyNever,
					SecurityContext:              &corev1.PodSecurityContext{},
					Containers: []corev1.Container{
						{
							Image:           "ghcr.io/grafana/k6-operator:latest-runner",
							ImagePullPolicy: "",
							Name:            "k6",
							Command: []string{
								"sh", "-c",
								"mkdir -p $(dirname /tmp/test.js.archived.tar) && k6 archive /test/test.js -O /tmp/test.js.archived.tar --out cloud 2> /tmp/k6logs && k6 inspect --execution-requirements /tmp/test.js.archived.tar 2> /tmp/k6logs && ! cat /tmp/k6logs | grep 'level=error'",
							},
							Env: []corev1.EnvVar{},
							EnvFrom: []corev1.EnvFromSource{
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "env",
										},
									},
								},
							},
							Resources:       corev1.ResourceRequirements{},
							VolumeMounts:    script.VolumeMount(),
							Ports:           []corev1.ContainerPort{{ContainerPort: 6565}},
							SecurityContext: &corev1.SecurityContext{},
						},
					},
					Volumes: script.Volume(),
				},
			},
		},
	}

	k6 := &v1alpha1.TestRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: v1alpha1.TestRunSpec{
			Script: v1alpha1.K6Script{
				ConfigMap: v1alpha1.K6Configmap{
					Name: "test",
					File: "test.js",
				},
			},
			Arguments: "--out cloud",
			Initializer: &v1alpha1.Pod{
				Metadata: v1alpha1.PodMetadata{
					Labels: map[string]string{
						"label1": "awesome",
					},
					Annotations: map[string]string{
						"awesomeAnnotation": "dope",
					},
				},
				EnvFrom: []corev1.EnvFromSource{
					{
						ConfigMapRef: &corev1.ConfigMapEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "env",
							},
						},
					},
				},
			},
		},
	}

	job, err := NewInitializerJob(k6, "--out cloud")
	if err != nil {
		t.Errorf("NewInitializerJob errored, got: %v", err)
	}

	if diff := deep.Equal(job, expectedOutcome); diff != nil {
		t.Error(diff)
	}
}
