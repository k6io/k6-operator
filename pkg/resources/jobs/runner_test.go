package jobs

import (
	"errors"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/util/intstr"

	deep "github.com/go-test/deep"
	"github.com/grafana/k6-operator/api/v1alpha1"
	"github.com/grafana/k6-operator/pkg/types"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// these are default values hard-coded in k6
var aggregationEnvVars = []corev1.EnvVar{
	{
		Name:  "K6_CLOUD_API_VERSION",
		Value: "2",
	}, {
		Name:  "K6_CLOUD_AGGREGATION_PERIOD",
		Value: "5s",
	}, {
		Name:  "K6_CLOUD_AGGREGATION_WAIT_PERIOD",
		Value: "3s",
	}, {
		Name:  "K6_CLOUD_METRIC_PUSH_INTERVAL",
		Value: "10s",
	}, {
		Name:  "K6_CLOUD_METRIC_PUSH_CONCURRENCY",
		Value: "10",
	},
}

func TestNewScriptVolumeClaim(t *testing.T) {
	expectedOutcome := &types.Script{
		Name:     "Test",
		Path:     "/test/",
		Filename: "thing.js",
		Type:     "VolumeClaim",
	}

	k6 := v1alpha1.TestRunSpec{
		Script: v1alpha1.K6Script{
			VolumeClaim: v1alpha1.K6VolumeClaim{
				Name: "Test",
				File: "thing.js",
			},
		},
	}

	script, err := k6.ParseScript()
	if err != nil {
		t.Errorf("NewScript with ConfigMap errored, got: %v, want: %v", err, expectedOutcome)
	}
	if !reflect.DeepEqual(script, expectedOutcome) {
		t.Errorf("NewScript with VolumeClaim failed to return expected output, got: %v, expected: %v", script, expectedOutcome)
	}
}

func TestNewScriptConfigMap(t *testing.T) {
	expectedOutcome := &types.Script{
		Name:     "Test",
		Path:     "/test/",
		Filename: "thing.js",
		Type:     "ConfigMap",
	}

	k6 := v1alpha1.TestRunSpec{
		Script: v1alpha1.K6Script{
			ConfigMap: v1alpha1.K6Configmap{
				Name: "Test",
				File: "thing.js",
			},
		},
	}

	script, err := k6.ParseScript()
	if err != nil {
		t.Errorf("NewScript with ConfigMap errored, got: %v, want: %v", err, expectedOutcome)
	}
	if !reflect.DeepEqual(script, expectedOutcome) {
		t.Errorf("NewScript with ConfigMap failed to return expected output, got: %v, expected: %v", script, expectedOutcome)
	}
}

func TestNewScriptLocalFile(t *testing.T) {

	expectedOutcome := &types.Script{
		Name:     "LocalFile",
		Path:     "/custom/",
		Filename: "my_test.js",
		Type:     "LocalFile",
	}

	k6 := v1alpha1.TestRunSpec{
		Script: v1alpha1.K6Script{
			LocalFile: "/custom/my_test.js",
		},
	}

	script, err := k6.ParseScript()
	if err != nil {
		t.Errorf("NewScript with LocalFile errored, got: %v, want: %v", err, expectedOutcome)
	}
	if !reflect.DeepEqual(script, expectedOutcome) {
		t.Errorf("NewScript with LocalFile failed to return expected output, got: %v, expected: %v", script, expectedOutcome)
	}
}

func TestNewScriptNoScript(t *testing.T) {
	k6 := v1alpha1.TestRunSpec{}

	script, err := k6.ParseScript()
	if err == nil && script != nil {
		t.Errorf("Expected Error from NewScript, got: %v, want: %v", err, errors.New("configMap, VolumeClaim or LocalFile not provided in script definition"))
	}
}

func TestNewVolumeSpecVolumeClaim(t *testing.T) {
	expectedOutcome := []corev1.Volume{
		corev1.Volume{
			Name: "k6-test-volume",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "test",
				},
			},
		},
	}

	script := &types.Script{
		Type: "VolumeClaim",
		Name: "test",
	}

	volumeSpec := script.Volume()
	if !reflect.DeepEqual(volumeSpec, expectedOutcome) {
		t.Errorf("VolumeSpec wasn't as expected, got: %v, expected: %v", volumeSpec, expectedOutcome)
	}
}

func TestNewVolumeSpecConfigMap(t *testing.T) {
	expectedOutcome := []corev1.Volume{
		corev1.Volume{
			Name: "k6-test-volume",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "test",
					},
				},
			},
		},
	}

	script := &types.Script{
		Type: "ConfigMap",
		Name: "test",
	}

	volumeSpec := script.Volume()
	if !reflect.DeepEqual(volumeSpec, expectedOutcome) {
		t.Errorf("VolumeSpec wasn't as expected, got: %v, expected: %v", volumeSpec, expectedOutcome)
	}
}

func TestNewVolumeSpecNoType(t *testing.T) {
	expectedOutcome := []corev1.Volume{}

	script := &types.Script{
		Name: "test",
	}

	volumeSpec := script.Volume()
	if !reflect.DeepEqual(volumeSpec, expectedOutcome) {
		t.Errorf("VolumeSpec wasn't as expected, got: %v, expected: %v", volumeSpec, expectedOutcome)
	}

}

func TestNewAntiAffinity(t *testing.T) {
	expectedOutcome := &corev1.Affinity{
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

	antiAffinity := newAntiAffinity()

	if !reflect.DeepEqual(antiAffinity, expectedOutcome) {
		t.Errorf("AntiAffinity returning unexpected values, got: %v, expected: %v", antiAffinity, expectedOutcome)
	}
}

func TestNewRunnerService(t *testing.T) {
	expectedOutcome := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service-1",
			Namespace: "test",
			Labels: map[string]string{
				"app":    "k6",
				"k6_cr":  "test",
				"runner": "true",
				"label1": "awesome",
			},
			Annotations: map[string]string{
				"awesomeAnnotation": "dope",
			}},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Name:     "http-api",
				Port:     6565,
				Protocol: "TCP",
			}},
			Selector: map[string]string{
				"job-name": "test-1",
			},
		},
	}

	k6 := &v1alpha1.TestRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: v1alpha1.TestRunSpec{
			Runner: v1alpha1.Pod{
				Metadata: v1alpha1.PodMetadata{
					Labels: map[string]string{
						"label1": "awesome",
					},
					Annotations: map[string]string{
						"awesomeAnnotation": "dope",
					},
				},
			},
		},
	}

	service, err := NewRunnerService(k6, 1)
	if err != nil {
		t.Errorf("NewRunnerService with errored, got: %v, want: %v", err, expectedOutcome)
	}
	if diff := deep.Equal(service, expectedOutcome); diff != nil {
		t.Errorf("NewRunnerService returned unexpected data, diff: %s", diff)
	}
}

func TestNewRunnerJob(t *testing.T) {
	script := &types.Script{
		Name:     "test",
		Filename: "thing.js",
		Type:     "ConfigMap",
	}

	var zero int64 = 0
	automountServiceAccountToken := true

	expectedLabels := map[string]string{
		"app":    "k6",
		"k6_cr":  "test",
		"runner": "true",
		"label1": "awesome",
	}

	expectedOutcome := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-1",
			Namespace: "test",
			Labels:    expectedLabels,
			Annotations: map[string]string{
				"awesomeAnnotation": "dope",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: new(int32),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: expectedLabels,
					Annotations: map[string]string{
						"awesomeAnnotation": "dope",
					},
				},
				Spec: corev1.PodSpec{
					Hostname:                     "test-1",
					RestartPolicy:                corev1.RestartPolicyNever,
					SecurityContext:              &corev1.PodSecurityContext{},
					Affinity:                     nil,
					NodeSelector:                 nil,
					Tolerations:                  nil,
					TopologySpreadConstraints:    nil,
					ServiceAccountName:           "default",
					AutomountServiceAccountToken: &automountServiceAccountToken,
					Containers: []corev1.Container{{
						Image:           "ghcr.io/grafana/k6-operator:latest-runner",
						ImagePullPolicy: corev1.PullNever,
						Name:            "k6",
						Command:         []string{"k6", "run", "--quiet", "/test/test.js", "--address=0.0.0.0:6565", "--paused", "--tag", "instance_id=1", "--tag", "job_name=test-1", "--tag", "base_job_name=test"},
						Env:             []corev1.EnvVar{},
						Resources:       corev1.ResourceRequirements{},
						VolumeMounts:    script.VolumeMount(),
						Ports:           []corev1.ContainerPort{{ContainerPort: 6565}},
						EnvFrom: []corev1.EnvFromSource{
							{
								ConfigMapRef: &corev1.ConfigMapEnvSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "env",
									},
								},
							},
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/v1/status",
									Port:   intstr.IntOrString{IntVal: 6565},
									Scheme: "HTTP",
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/v1/status",
									Port:   intstr.IntOrString{IntVal: 6565},
									Scheme: "HTTP",
								},
							},
						},
						SecurityContext: &corev1.SecurityContext{},
					}},
					TerminationGracePeriodSeconds: &zero,
					Volumes:                       script.Volume(),
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
			Runner: v1alpha1.Pod{
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
				ImagePullPolicy: corev1.PullNever,
			},
		},
	}

	job, err := NewRunnerJob(k6, 1, "")
	if err != nil {
		t.Errorf("NewRunnerJob errored, got: %v", err)
	}
	if diff := deep.Equal(job, expectedOutcome); diff != nil {
		t.Errorf("NewRunnerJob returned unexpected data, diff: %s", diff)
	}
}

func TestNewRunnerJobNoisy(t *testing.T) {
	script := &types.Script{
		Name:     "test",
		Filename: "thing.js",
		Type:     "ConfigMap",
	}

	var zero int64 = 0
	automountServiceAccountToken := true

	expectedLabels := map[string]string{
		"app":    "k6",
		"k6_cr":  "test",
		"runner": "true",
		"label1": "awesome",
	}

	expectedOutcome := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-1",
			Namespace: "test",
			Labels:    expectedLabels,
			Annotations: map[string]string{
				"awesomeAnnotation": "dope",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: new(int32),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: expectedLabels,
					Annotations: map[string]string{
						"awesomeAnnotation": "dope",
					},
				},
				Spec: corev1.PodSpec{
					Hostname:                     "test-1",
					RestartPolicy:                corev1.RestartPolicyNever,
					Affinity:                     nil,
					NodeSelector:                 nil,
					Tolerations:                  nil,
					TopologySpreadConstraints:    nil,
					ServiceAccountName:           "default",
					AutomountServiceAccountToken: &automountServiceAccountToken,
					SecurityContext:              &corev1.PodSecurityContext{},
					Containers: []corev1.Container{{
						Image:           "ghcr.io/grafana/k6-operator:latest-runner",
						ImagePullPolicy: "",
						Name:            "k6",
						Command:         []string{"k6", "run", "/test/test.js", "--address=0.0.0.0:6565", "--paused", "--tag", "instance_id=1", "--tag", "job_name=test-1", "--tag", "base_job_name=test"},
						Env:             []corev1.EnvVar{},
						Resources:       corev1.ResourceRequirements{},
						VolumeMounts:    script.VolumeMount(),
						Ports:           []corev1.ContainerPort{{ContainerPort: 6565}},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/v1/status",
									Port:   intstr.IntOrString{IntVal: 6565},
									Scheme: "HTTP",
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/v1/status",
									Port:   intstr.IntOrString{IntVal: 6565},
									Scheme: "HTTP",
								},
							},
						},
						SecurityContext: &corev1.SecurityContext{},
					}},
					TerminationGracePeriodSeconds: &zero,
					Volumes:                       script.Volume(),
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
			Quiet: "false",
			Script: v1alpha1.K6Script{
				ConfigMap: v1alpha1.K6Configmap{
					Name: "test",
					File: "test.js",
				},
			},
			Runner: v1alpha1.Pod{
				Metadata: v1alpha1.PodMetadata{
					Labels: map[string]string{
						"label1": "awesome",
					},
					Annotations: map[string]string{
						"awesomeAnnotation": "dope",
					},
				},
			},
		},
	}

	job, err := NewRunnerJob(k6, 1, "")
	if err != nil {
		t.Errorf("NewRunnerJob errored, got: %v", err)
	}
	if diff := deep.Equal(job, expectedOutcome); diff != nil {
		t.Errorf("NewRunnerJob returned unexpected data, diff: %s", diff)
	}
}

func TestNewRunnerJobUnpaused(t *testing.T) {
	script := &types.Script{
		Name:     "test",
		Filename: "thing.js",
		Type:     "ConfigMap",
	}

	var zero int64 = 0
	automountServiceAccountToken := true

	expectedLabels := map[string]string{
		"app":    "k6",
		"k6_cr":  "test",
		"runner": "true",
		"label1": "awesome",
	}

	expectedOutcome := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-1",
			Namespace: "test",
			Labels:    expectedLabels,
			Annotations: map[string]string{
				"awesomeAnnotation": "dope",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: new(int32),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: expectedLabels,
					Annotations: map[string]string{
						"awesomeAnnotation": "dope",
					},
				},
				Spec: corev1.PodSpec{
					Hostname:                     "test-1",
					RestartPolicy:                corev1.RestartPolicyNever,
					Affinity:                     nil,
					NodeSelector:                 nil,
					Tolerations:                  nil,
					TopologySpreadConstraints:    nil,
					ServiceAccountName:           "default",
					AutomountServiceAccountToken: &automountServiceAccountToken,
					SecurityContext:              &corev1.PodSecurityContext{},
					Containers: []corev1.Container{{
						Image:           "ghcr.io/grafana/k6-operator:latest-runner",
						ImagePullPolicy: "",
						Name:            "k6",
						Command:         []string{"k6", "run", "--quiet", "/test/test.js", "--address=0.0.0.0:6565", "--tag", "instance_id=1", "--tag", "job_name=test-1", "--tag", "base_job_name=test"},
						Env:             []corev1.EnvVar{},
						Resources:       corev1.ResourceRequirements{},
						VolumeMounts:    script.VolumeMount(),
						Ports:           []corev1.ContainerPort{{ContainerPort: 6565}},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/v1/status",
									Port:   intstr.IntOrString{IntVal: 6565},
									Scheme: "HTTP",
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/v1/status",
									Port:   intstr.IntOrString{IntVal: 6565},
									Scheme: "HTTP",
								},
							},
						},
						SecurityContext: &corev1.SecurityContext{},
					}},
					TerminationGracePeriodSeconds: &zero,
					Volumes:                       script.Volume(),
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
			Paused: "false",
			Script: v1alpha1.K6Script{
				ConfigMap: v1alpha1.K6Configmap{
					Name: "test",
					File: "test.js",
				},
			},
			Runner: v1alpha1.Pod{
				Metadata: v1alpha1.PodMetadata{
					Labels: map[string]string{
						"label1": "awesome",
					},
					Annotations: map[string]string{
						"awesomeAnnotation": "dope",
					},
				},
			},
		},
	}

	job, err := NewRunnerJob(k6, 1, "")
	if err != nil {
		t.Errorf("NewRunnerJob errored, got: %v", err)
	}
	if diff := deep.Equal(job, expectedOutcome); diff != nil {
		t.Errorf("NewRunnerJob returned unexpected data, diff: %s", diff)
	}
}

func TestNewRunnerJobArguments(t *testing.T) {
	script := &types.Script{
		Name:     "test",
		Filename: "thing.js",
		Type:     "ConfigMap",
	}

	var zero int64 = 0
	automountServiceAccountToken := true

	expectedLabels := map[string]string{
		"app":    "k6",
		"k6_cr":  "test",
		"runner": "true",
		"label1": "awesome",
	}

	expectedOutcome := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-1",
			Namespace: "test",
			Labels:    expectedLabels,
			Annotations: map[string]string{
				"awesomeAnnotation": "dope",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: new(int32),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: expectedLabels,
					Annotations: map[string]string{
						"awesomeAnnotation": "dope",
					},
				},
				Spec: corev1.PodSpec{
					Hostname:                     "test-1",
					RestartPolicy:                corev1.RestartPolicyNever,
					Affinity:                     nil,
					NodeSelector:                 nil,
					Tolerations:                  nil,
					TopologySpreadConstraints:    nil,
					ServiceAccountName:           "default",
					AutomountServiceAccountToken: &automountServiceAccountToken,
					SecurityContext:              &corev1.PodSecurityContext{},
					Containers: []corev1.Container{{
						Image:           "ghcr.io/grafana/k6-operator:latest-runner",
						ImagePullPolicy: "",
						Name:            "k6",
						Command:         []string{"k6", "run", "--quiet", "--cool-thing", "/test/test.js", "--address=0.0.0.0:6565", "--paused", "--tag", "instance_id=1", "--tag", "job_name=test-1", "--tag", "base_job_name=test"},
						Env:             []corev1.EnvVar{},
						Resources:       corev1.ResourceRequirements{},
						VolumeMounts:    script.VolumeMount(),
						Ports:           []corev1.ContainerPort{{ContainerPort: 6565}},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/v1/status",
									Port:   intstr.IntOrString{IntVal: 6565},
									Scheme: "HTTP",
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/v1/status",
									Port:   intstr.IntOrString{IntVal: 6565},
									Scheme: "HTTP",
								},
							},
						},
						SecurityContext: &corev1.SecurityContext{},
					}},
					TerminationGracePeriodSeconds: &zero,
					Volumes:                       script.Volume(),
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
			Arguments: "--cool-thing",
			Script: v1alpha1.K6Script{
				ConfigMap: v1alpha1.K6Configmap{
					Name: "test",
					File: "test.js",
				},
			},
			Runner: v1alpha1.Pod{
				Metadata: v1alpha1.PodMetadata{
					Labels: map[string]string{
						"label1": "awesome",
					},
					Annotations: map[string]string{
						"awesomeAnnotation": "dope",
					},
				},
			},
		},
	}

	job, err := NewRunnerJob(k6, 1, "")
	if err != nil {
		t.Errorf("NewRunnerJob errored, got: %v", err)
	}
	if diff := deep.Equal(job, expectedOutcome); diff != nil {
		t.Errorf("NewRunnerJob returned unexpected data, diff: %s", diff)
	}
}

func TestNewRunnerJobServiceAccount(t *testing.T) {
	script := &types.Script{
		Name:     "test",
		Filename: "thing.js",
		Type:     "ConfigMap",
	}

	var zero int64 = 0
	automountServiceAccountToken := true

	expectedLabels := map[string]string{
		"app":    "k6",
		"k6_cr":  "test",
		"runner": "true",
		"label1": "awesome",
	}

	expectedOutcome := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-1",
			Namespace: "test",
			Labels:    expectedLabels,
			Annotations: map[string]string{
				"awesomeAnnotation": "dope",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: new(int32),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: expectedLabels,
					Annotations: map[string]string{
						"awesomeAnnotation": "dope",
					},
				},
				Spec: corev1.PodSpec{
					Hostname:                     "test-1",
					RestartPolicy:                corev1.RestartPolicyNever,
					Affinity:                     nil,
					NodeSelector:                 nil,
					Tolerations:                  nil,
					TopologySpreadConstraints:    nil,
					ServiceAccountName:           "test",
					AutomountServiceAccountToken: &automountServiceAccountToken,
					SecurityContext:              &corev1.PodSecurityContext{},
					Containers: []corev1.Container{{
						Image:           "ghcr.io/grafana/k6-operator:latest-runner",
						ImagePullPolicy: "",
						Name:            "k6",
						Command:         []string{"k6", "run", "--quiet", "/test/test.js", "--address=0.0.0.0:6565", "--paused", "--tag", "instance_id=1", "--tag", "job_name=test-1", "--tag", "base_job_name=test"},
						Env:             []corev1.EnvVar{},
						Resources:       corev1.ResourceRequirements{},
						VolumeMounts:    script.VolumeMount(),
						Ports:           []corev1.ContainerPort{{ContainerPort: 6565}},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/v1/status",
									Port:   intstr.IntOrString{IntVal: 6565},
									Scheme: "HTTP",
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/v1/status",
									Port:   intstr.IntOrString{IntVal: 6565},
									Scheme: "HTTP",
								},
							},
						},
						SecurityContext: &corev1.SecurityContext{},
					}},
					TerminationGracePeriodSeconds: &zero,
					Volumes:                       script.Volume(),
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
			Runner: v1alpha1.Pod{
				ServiceAccountName: "test",
				Metadata: v1alpha1.PodMetadata{
					Labels: map[string]string{
						"label1": "awesome",
					},
					Annotations: map[string]string{
						"awesomeAnnotation": "dope",
					},
				},
			},
		},
	}

	job, err := NewRunnerJob(k6, 1, "")
	if err != nil {
		t.Errorf("NewRunnerJob errored, got: %v", err)
	}
	if diff := deep.Equal(job, expectedOutcome); diff != nil {
		t.Errorf("NewRunnerJob returned unexpected data, diff: %s", diff)
	}
}

func TestNewRunnerJobIstio(t *testing.T) {
	script := &types.Script{
		Name:     "test",
		Filename: "thing.js",
		Type:     "ConfigMap",
	}

	var zero int64 = 0
	automountServiceAccountToken := true

	expectedLabels := map[string]string{
		"app":    "k6",
		"k6_cr":  "test",
		"runner": "true",
		"label1": "awesome",
	}

	expectedOutcome := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-1",
			Namespace: "test",
			Labels:    expectedLabels,
			Annotations: map[string]string{
				"awesomeAnnotation": "dope",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: new(int32),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: expectedLabels,
					Annotations: map[string]string{
						"awesomeAnnotation": "dope",
					},
				},
				Spec: corev1.PodSpec{
					Hostname:                     "test-1",
					RestartPolicy:                corev1.RestartPolicyNever,
					Affinity:                     nil,
					NodeSelector:                 nil,
					Tolerations:                  nil,
					TopologySpreadConstraints:    nil,
					ServiceAccountName:           "default",
					AutomountServiceAccountToken: &automountServiceAccountToken,
					SecurityContext:              &corev1.PodSecurityContext{},
					Containers: []corev1.Container{{
						Image:           "ghcr.io/grafana/k6-operator:latest-runner",
						ImagePullPolicy: "",
						Name:            "k6",
						Command:         []string{"scuttle", "k6", "run", "--quiet", "/test/test.js", "--address=0.0.0.0:6565", "--paused", "--tag", "instance_id=1", "--tag", "job_name=test-1", "--tag", "base_job_name=test"},
						Env: []corev1.EnvVar{
							{
								Name:  "ENVOY_ADMIN_API",
								Value: "http://127.0.0.1:15000",
							},
							{
								Name:  "ISTIO_QUIT_API",
								Value: "http://127.0.0.1:15020",
							},
							{
								Name:  "WAIT_FOR_ENVOY_TIMEOUT",
								Value: "15",
							},
						},
						Resources:    corev1.ResourceRequirements{},
						VolumeMounts: script.VolumeMount(),
						Ports:        []corev1.ContainerPort{{ContainerPort: 6565}},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/v1/status",
									Port:   intstr.IntOrString{IntVal: 6565},
									Scheme: "HTTP",
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/v1/status",
									Port:   intstr.IntOrString{IntVal: 6565},
									Scheme: "HTTP",
								},
							},
						},
						SecurityContext: &corev1.SecurityContext{},
					}},
					TerminationGracePeriodSeconds: &zero,
					Volumes:                       script.Volume(),
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
			Scuttle: v1alpha1.K6Scuttle{
				Enabled: "true",
			},
			Script: v1alpha1.K6Script{
				ConfigMap: v1alpha1.K6Configmap{
					Name: "test",
					File: "test.js",
				},
			},
			Runner: v1alpha1.Pod{
				Metadata: v1alpha1.PodMetadata{
					Labels: map[string]string{
						"label1": "awesome",
					},
					Annotations: map[string]string{
						"awesomeAnnotation": "dope",
					},
				},
			},
		},
	}

	job, err := NewRunnerJob(k6, 1, "")
	if err != nil {
		t.Errorf("NewRunnerJob errored, got: %v", err)
	}
	if diff := deep.Equal(job, expectedOutcome); diff != nil {
		t.Errorf("NewRunnerJob returned unexpected data, diff: %s", diff)
	}
}

func TestNewRunnerJobCloud(t *testing.T) {
	script := &types.Script{
		Name:     "test",
		Filename: "thing.js",
		Type:     "ConfigMap",
	}

	var zero int64 = 0
	automountServiceAccountToken := true

	expectedLabels := map[string]string{
		"app":    "k6",
		"k6_cr":  "test",
		"runner": "true",
	}

	expectedOutcome := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-1",
			Namespace: "test",
			Labels:    expectedLabels,
			Annotations: map[string]string{
				"awesomeAnnotation": "dope",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: new(int32),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: expectedLabels,
					Annotations: map[string]string{
						"awesomeAnnotation": "dope",
					},
				},
				Spec: corev1.PodSpec{
					Hostname:                     "test-1",
					RestartPolicy:                corev1.RestartPolicyNever,
					Affinity:                     nil,
					NodeSelector:                 nil,
					Tolerations:                  nil,
					TopologySpreadConstraints:    nil,
					ServiceAccountName:           "default",
					SecurityContext:              &corev1.PodSecurityContext{},
					AutomountServiceAccountToken: &automountServiceAccountToken,
					Containers: []corev1.Container{{
						Image:           "ghcr.io/grafana/k6-operator:latest-runner",
						ImagePullPolicy: "",
						Name:            "k6",
						Command:         []string{"k6", "run", "--quiet", "--out", "cloud", "/test/test.js", "--address=0.0.0.0:6565", "--paused", "--tag", "instance_id=1", "--tag", "job_name=test-1", "--tag", "base_job_name=test"},
						Env: append(aggregationEnvVars,
							corev1.EnvVar{
								Name:  "K6_CLOUD_PUSH_REF_ID",
								Value: "testrunid",
							},
							corev1.EnvVar{
								Name:  "K6_CLOUD_TOKEN",
								Value: "token",
							},
						),
						Resources:    corev1.ResourceRequirements{},
						VolumeMounts: script.VolumeMount(),
						Ports:        []corev1.ContainerPort{{ContainerPort: 6565}},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/v1/status",
									Port:   intstr.IntOrString{IntVal: 6565},
									Scheme: "HTTP",
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/v1/status",
									Port:   intstr.IntOrString{IntVal: 6565},
									Scheme: "HTTP",
								},
							},
						},
						SecurityContext: &corev1.SecurityContext{},
					}},
					TerminationGracePeriodSeconds: &zero,
					Volumes:                       script.Volume(),
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
			Runner: v1alpha1.Pod{
				Metadata: v1alpha1.PodMetadata{
					Annotations: map[string]string{
						"awesomeAnnotation": "dope",
					},
				},
			},
		},
		// Since this test only creates a runner's spec so
		// testrunid has to be set hard-coded here.
		Status: v1alpha1.TestRunStatus{
			TestRunID:       "testrunid",
			AggregationVars: "2|5s|3s|10s|10",
		},
	}

	job, err := NewRunnerJob(k6, 1, "token")
	if err != nil {
		t.Errorf("NewRunnerJob errored, got: %v", err)
	}
	if diff := deep.Equal(job, expectedOutcome); diff != nil {
		t.Errorf("NewRunnerJob returned unexpected data, diff: %s", diff)
	}
}

func TestNewRunnerJobLocalFile(t *testing.T) {
	script := &types.Script{
		Name:     "test",
		Filename: "/test/test.js",
		Type:     "LocalFile",
	}

	var zero int64 = 0
	automountServiceAccountToken := true

	expectedLabels := map[string]string{
		"app":    "k6",
		"k6_cr":  "test",
		"runner": "true",
		"label1": "awesome",
	}
	expectedOutcome := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-1",
			Namespace: "test",
			Labels:    expectedLabels,
			Annotations: map[string]string{
				"awesomeAnnotation": "dope",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: new(int32),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: expectedLabels,
					Annotations: map[string]string{
						"awesomeAnnotation": "dope",
					},
				},
				Spec: corev1.PodSpec{
					Hostname:                     "test-1",
					RestartPolicy:                corev1.RestartPolicyNever,
					Affinity:                     nil,
					NodeSelector:                 nil,
					Tolerations:                  nil,
					TopologySpreadConstraints:    nil,
					ServiceAccountName:           "default",
					AutomountServiceAccountToken: &automountServiceAccountToken,
					SecurityContext:              &corev1.PodSecurityContext{},
					Containers: []corev1.Container{{
						Image:           "ghcr.io/grafana/k6-operator:latest-runner",
						ImagePullPolicy: "",
						Name:            "k6",
						Command:         []string{"sh", "-c", "if [ ! -f /test/test.js ]; then echo \"LocalFile not found exiting...\"; exit 1; fi;\nk6 run --quiet /test/test.js --address=0.0.0.0:6565 --paused --tag instance_id=1 --tag job_name=test-1 --tag base_job_name=test"},
						Env:             []corev1.EnvVar{},
						Resources:       corev1.ResourceRequirements{},
						VolumeMounts:    script.VolumeMount(),
						Ports:           []corev1.ContainerPort{{ContainerPort: 6565}},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/v1/status",
									Port:   intstr.IntOrString{IntVal: 6565},
									Scheme: "HTTP",
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/v1/status",
									Port:   intstr.IntOrString{IntVal: 6565},
									Scheme: "HTTP",
								},
							},
						},
						SecurityContext: &corev1.SecurityContext{},
					}},
					TerminationGracePeriodSeconds: &zero,
					Volumes:                       script.Volume(),
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
			Scuttle: v1alpha1.K6Scuttle{
				Enabled: "false",
			},
			Script: v1alpha1.K6Script{
				LocalFile: "/test/test.js",
			},
			Runner: v1alpha1.Pod{
				Metadata: v1alpha1.PodMetadata{
					Labels: map[string]string{
						"label1": "awesome",
					},
					Annotations: map[string]string{
						"awesomeAnnotation": "dope",
					},
				},
			},
		},
	}

	job, err := NewRunnerJob(k6, 1, "")
	if err != nil {
		t.Errorf("NewRunnerJob errored, got: %v", err)
	}
	if diff := deep.Equal(job, expectedOutcome); diff != nil {
		t.Errorf("NewRunnerJob returned unexpected data, diff: %s", diff)
	}
}

func TestNewRunnerJobWithInitContainer(t *testing.T) {
	script := &types.Script{
		Name:     "test",
		Filename: "thing.js",
		Type:     "ConfigMap",
	}

	var zero int64 = 0
	automountServiceAccountToken := true

	expectedLabels := map[string]string{
		"app":    "k6",
		"k6_cr":  "test",
		"runner": "true",
		"label1": "awesome",
	}

	expectedOutcome := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-1",
			Namespace: "test",
			Labels:    expectedLabels,
			Annotations: map[string]string{
				"awesomeAnnotation": "dope",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: new(int32),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: expectedLabels,
					Annotations: map[string]string{
						"awesomeAnnotation": "dope",
					},
				},
				Spec: corev1.PodSpec{
					Hostname:                     "test-1",
					RestartPolicy:                corev1.RestartPolicyNever,
					SecurityContext:              &corev1.PodSecurityContext{},
					Affinity:                     nil,
					NodeSelector:                 nil,
					Tolerations:                  nil,
					TopologySpreadConstraints:    nil,
					ServiceAccountName:           "default",
					AutomountServiceAccountToken: &automountServiceAccountToken,
					InitContainers: []corev1.Container{
						{
							Name:            "k6-init-0",
							Image:           "busybox:1.28",
							Command:         []string{"sh", "-c", "cat /test/test.js"},
							VolumeMounts:    script.VolumeMount(),
							ImagePullPolicy: corev1.PullNever,
							EnvFrom: []corev1.EnvFromSource{
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "env",
										},
									},
								},
							},
							SecurityContext: &corev1.SecurityContext{},
						},
					},
					Containers: []corev1.Container{{
						Image:           "ghcr.io/grafana/k6-operator:latest-runner",
						ImagePullPolicy: corev1.PullNever,
						Name:            "k6",
						Command:         []string{"k6", "run", "--quiet", "/test/test.js", "--address=0.0.0.0:6565", "--paused", "--tag", "instance_id=1", "--tag", "job_name=test-1", "--tag", "base_job_name=test"},
						Env:             []corev1.EnvVar{},
						Resources:       corev1.ResourceRequirements{},
						VolumeMounts:    script.VolumeMount(),
						Ports:           []corev1.ContainerPort{{ContainerPort: 6565}},
						EnvFrom: []corev1.EnvFromSource{
							{
								ConfigMapRef: &corev1.ConfigMapEnvSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "env",
									},
								},
							},
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/v1/status",
									Port:   intstr.IntOrString{IntVal: 6565},
									Scheme: "HTTP",
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/v1/status",
									Port:   intstr.IntOrString{IntVal: 6565},
									Scheme: "HTTP",
								},
							},
						},
						SecurityContext: &corev1.SecurityContext{},
					}},
					TerminationGracePeriodSeconds: &zero,
					Volumes:                       script.Volume(),
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
			Runner: v1alpha1.Pod{
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
				ImagePullPolicy: corev1.PullNever,
				InitContainers: []v1alpha1.InitContainer{
					{
						Image:   "busybox:1.28",
						Command: []string{"sh", "-c", "cat /test/test.js"},
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
			},
		},
	}

	job, err := NewRunnerJob(k6, 1, "")
	if err != nil {
		t.Errorf("NewRunnerJob errored, got: %v", err)
	}
	if diff := deep.Equal(job, expectedOutcome); diff != nil {
		t.Errorf("NewRunnerJob returned unexpected data, diff: %s", diff)
	}
}

func TestNewRunnerJobWithVolume(t *testing.T) {
	script := &types.Script{
		Name:     "test",
		Filename: "thing.js",
		Type:     "ConfigMap",
	}

	var zero int64 = 0
	automountServiceAccountToken := true

	expectedLabels := map[string]string{
		"app":    "k6",
		"k6_cr":  "test",
		"runner": "true",
		"label1": "awesome",
	}

	expectedVolumes := append(script.Volume(), corev1.Volume{
		Name: "k6-test-volume",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})

	expectedVolumeMounts := append(script.VolumeMount(), corev1.VolumeMount{
		Name:      "k6-test-volume",
		MountPath: "/test/location",
	})

	expectedOutcome := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-1",
			Namespace: "test",
			Labels:    expectedLabels,
			Annotations: map[string]string{
				"awesomeAnnotation": "dope",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: new(int32),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: expectedLabels,
					Annotations: map[string]string{
						"awesomeAnnotation": "dope",
					},
				},
				Spec: corev1.PodSpec{
					Hostname:                     "test-1",
					RestartPolicy:                corev1.RestartPolicyNever,
					SecurityContext:              &corev1.PodSecurityContext{},
					Affinity:                     nil,
					NodeSelector:                 nil,
					Tolerations:                  nil,
					TopologySpreadConstraints:    nil,
					ServiceAccountName:           "default",
					AutomountServiceAccountToken: &automountServiceAccountToken,
					InitContainers: []corev1.Container{
						{
							Name:            "k6-init-0",
							Image:           "busybox:1.28",
							Command:         []string{"sh", "-c", "cat /test/test.js"},
							VolumeMounts:    expectedVolumeMounts,
							ImagePullPolicy: corev1.PullNever,
							EnvFrom: []corev1.EnvFromSource{
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "env",
										},
									},
								},
							},
							SecurityContext: &corev1.SecurityContext{},
						},
					},
					Containers: []corev1.Container{{
						Image:           "ghcr.io/grafana/k6-operator:latest-runner",
						ImagePullPolicy: corev1.PullNever,
						Name:            "k6",
						Command:         []string{"k6", "run", "--quiet", "/test/test.js", "--address=0.0.0.0:6565", "--paused", "--tag", "instance_id=1", "--tag", "job_name=test-1", "--tag", "base_job_name=test"},
						Env:             []corev1.EnvVar{},
						Resources:       corev1.ResourceRequirements{},
						VolumeMounts:    expectedVolumeMounts,
						Ports:           []corev1.ContainerPort{{ContainerPort: 6565}},
						EnvFrom: []corev1.EnvFromSource{
							{
								ConfigMapRef: &corev1.ConfigMapEnvSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "env",
									},
								},
							},
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/v1/status",
									Port:   intstr.IntOrString{IntVal: 6565},
									Scheme: "HTTP",
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/v1/status",
									Port:   intstr.IntOrString{IntVal: 6565},
									Scheme: "HTTP",
								},
							},
						},
						SecurityContext: &corev1.SecurityContext{},
					}},
					TerminationGracePeriodSeconds: &zero,
					Volumes:                       expectedVolumes,
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
			Runner: v1alpha1.Pod{
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
				ImagePullPolicy: corev1.PullNever,
				InitContainers: []v1alpha1.InitContainer{
					{
						Image:   "busybox:1.28",
						Command: []string{"sh", "-c", "cat /test/test.js"},
						EnvFrom: []corev1.EnvFromSource{
							{
								ConfigMapRef: &corev1.ConfigMapEnvSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "env",
									},
								},
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							corev1.VolumeMount{
								Name:      "k6-test-volume",
								MountPath: "/test/location",
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					corev1.Volume{
						Name: "k6-test-volume",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					corev1.VolumeMount{
						Name:      "k6-test-volume",
						MountPath: "/test/location",
					},
				},
			},
		},
	}

	job, err := NewRunnerJob(k6, 1, "")
	if err != nil {
		t.Errorf("NewRunnerJob errored, got: %v", err)
	}
	if diff := deep.Equal(job, expectedOutcome); diff != nil {
		t.Errorf("NewRunnerJob returned unexpected data, diff: %s", diff)
	}
}

func TestNewRunnerJobPLZTestRun(t *testing.T) {
	// NewRunnerJob does not validate the type of Script for
	// internal consistency (like in PLZ case) so it can be anything.
	script := &types.Script{
		Name:     "test",
		Filename: "thing.js",
		Type:     "ConfigMap",
	}

	var zero int64 = 0
	automountServiceAccountToken := true

	expectedLabels := map[string]string{
		"app":    "k6",
		"k6_cr":  "test",
		"runner": "true",
		"label1": "awesome",
	}

	expectedOutcome := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-1",
			Namespace: "test",
			Labels:    expectedLabels,
			Annotations: map[string]string{
				"awesomeAnnotation": "dope",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: new(int32),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: expectedLabels,
					Annotations: map[string]string{
						"awesomeAnnotation": "dope",
					},
				},
				Spec: corev1.PodSpec{
					Hostname:                     "test-1",
					RestartPolicy:                corev1.RestartPolicyNever,
					SecurityContext:              &corev1.PodSecurityContext{},
					Affinity:                     nil,
					NodeSelector:                 nil,
					Tolerations:                  nil,
					TopologySpreadConstraints:    nil,
					ServiceAccountName:           "default",
					AutomountServiceAccountToken: &automountServiceAccountToken,
					Containers: []corev1.Container{{
						Image:           "ghcr.io/grafana/k6-operator:latest-runner",
						ImagePullPolicy: corev1.PullNever,
						Name:            "k6",
						Command:         []string{"k6", "run", "--quiet", "/test/test.js", "--address=0.0.0.0:6565", "--paused", "--tag", "instance_id=1", "--tag", "job_name=test-1", "--tag", "base_job_name=test", "--no-setup", "--no-teardown", "--linger"},
						Env:             []corev1.EnvVar{},
						Resources:       corev1.ResourceRequirements{},
						VolumeMounts:    script.VolumeMount(),
						Ports:           []corev1.ContainerPort{{ContainerPort: 6565}},
						EnvFrom: []corev1.EnvFromSource{
							{
								ConfigMapRef: &corev1.ConfigMapEnvSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "env",
									},
								},
							},
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/v1/status",
									Port:   intstr.IntOrString{IntVal: 6565},
									Scheme: "HTTP",
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/v1/status",
									Port:   intstr.IntOrString{IntVal: 6565},
									Scheme: "HTTP",
								},
							},
						},
						SecurityContext: &corev1.SecurityContext{},
					}},
					TerminationGracePeriodSeconds: &zero,
					Volumes:                       script.Volume(),
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
			Runner: v1alpha1.Pod{
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
				ImagePullPolicy: corev1.PullNever,
			},
		},
		Status: v1alpha1.TestRunStatus{
			Conditions: []metav1.Condition{
				{
					Type:               v1alpha1.CloudPLZTestRun,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
				},
			},
		},
	}

	job, err := NewRunnerJob(k6, 1, "")
	if err != nil {
		t.Errorf("NewRunnerJob errored, got: %v", err)
	}
	if diff := deep.Equal(job, expectedOutcome); diff != nil {
		t.Errorf("NewRunnerJob returned unexpected data, diff: %s", diff)
	}
}
