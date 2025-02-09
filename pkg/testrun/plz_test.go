package testrun

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-test/deep"
	"github.com/grafana/k6-operator/api/v1alpha1"
	"github.com/grafana/k6-operator/pkg/cloud"
	"github.com/grafana/k6-operator/pkg/resources/containers"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_NewPLZTestRun(t *testing.T) {
	var (
		mainIngest = "https://ingest.k6.io"

		volumeMount = corev1.VolumeMount{
			Name:      "archive-volume",
			MountPath: "/test",
		}
		// zero-values test run definition
		defaultTestRun = v1alpha1.TestRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: TestName("0"),
			},
			Spec: v1alpha1.TestRunSpec{
				Runner: v1alpha1.Pod{
					Volumes: []corev1.Volume{{
						Name: "archive-volume",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
					},
					VolumeMounts: []corev1.VolumeMount{volumeMount},
					InitContainers: []v1alpha1.InitContainer{
						containers.NewS3InitContainer(
							"",
							"ghcr.io/grafana/k6-operator:latest-starter",
							volumeMount,
						),
					},
					Env: []corev1.EnvVar{{
						Name:  "K6_CLOUD_HOST",
						Value: mainIngest,
					}},
				},
				Script: v1alpha1.K6Script{
					LocalFile: "/test/archive.tar",
				},
				Parallelism: int32(0),
				Separate:    false,
				Arguments:   "--out cloud --no-thresholds --log-output=loki=https://cloudlogs.k6.io/api/v1/push,label.lz=,label.test_run_id=0,header.Authorization=\"Token token\"",
				Cleanup:     v1alpha1.Cleanup("post"),

				TestRunID: "0",
			},
		}

		// non-empty values to use int test cases
		someToken        = "some-token"
		someSA           = "some-service-account"
		someNodeSelector = map[string]string{"foo": "bar"}
		someNS           = "some-ns"
		resourceLimits   = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("200m"),
			corev1.ResourceMemory: resource.MustParse("1G"),
		}
		tolerations = []corev1.Toleration{
			{
				Key:      "someKey",
				Operator: corev1.TolerationOpEqual,
				Value:    "someValue",
				Effect:   corev1.TaintEffectNoSchedule,
			},
		}
		someTestRunID   = 6543
		someRunnerImage = "grafana/k6:0.52.0"
		someInstances   = 10
		someArchiveURL  = "https://foo.s3.amazonaws.com"
		someEnvVars     = map[string]string{
			"ENV": "VALUE",
			"foo": "bar",
		}

		// TestRuns expected in different cases;
		// see how they are populated below
		requiredFieldsTestRun = defaultTestRun
		optionalFieldsTestRun = defaultTestRun //nolint:ineffassign
		cloudFieldsTestRun    = defaultTestRun //nolint:ineffassign
		cloudEnvVarsTestRun   = defaultTestRun //nolint:ineffassign
	)

	// populate TestRuns for different test cases

	requiredFieldsTestRun.Spec.Token = someToken
	requiredFieldsTestRun.Spec.Runner.Resources.Limits = resourceLimits

	optionalFieldsTestRun = requiredFieldsTestRun // build up on top of required field case
	optionalFieldsTestRun.Namespace = someNS
	optionalFieldsTestRun.Spec.Runner.ServiceAccountName = someSA
	optionalFieldsTestRun.Spec.Runner.NodeSelector = someNodeSelector
	optionalFieldsTestRun.Spec.Runner.Tolerations = tolerations
	optionalFieldsTestRun.Spec.Starter.ServiceAccountName = someSA
	optionalFieldsTestRun.Spec.Starter.NodeSelector = someNodeSelector
	optionalFieldsTestRun.Spec.Starter.Tolerations = tolerations

	cloudFieldsTestRun = requiredFieldsTestRun // build up on top of required field case
	cloudFieldsTestRun.ObjectMeta.Name = TestName(fmt.Sprintf("%d", someTestRunID))
	cloudFieldsTestRun.Spec.TestRunID = fmt.Sprintf("%d", someTestRunID)
	cloudFieldsTestRun.Spec.Arguments = strings.Replace(requiredFieldsTestRun.Spec.Arguments,
		"test_run_id=0",
		fmt.Sprintf("test_run_id=%d", someTestRunID),
		1)
	cloudFieldsTestRun.Spec.Runner.InitContainers = []v1alpha1.InitContainer{
		containers.NewS3InitContainer(
			someArchiveURL,
			"ghcr.io/grafana/k6-operator:latest-starter",
			volumeMount,
		),
	}
	cloudFieldsTestRun.Spec.Runner.Image = someRunnerImage
	cloudFieldsTestRun.Spec.Parallelism = int32(someInstances)

	cloudEnvVarsTestRun = cloudFieldsTestRun // build up on top of cloud fields case
	cloudEnvVarsTestRun.Spec.Runner.Env = []corev1.EnvVar{
		{
			Name:  "ENV",
			Value: "VALUE",
		},
		{
			Name:  "foo",
			Value: "bar",
		},
		{
			Name:  "K6_CLOUD_HOST",
			Value: mainIngest,
		},
	}

	testCases := []struct {
		name      string
		plz       *v1alpha1.PrivateLoadZone
		cloudData *cloud.TestRunData
		ingestUrl string
		expected  *v1alpha1.TestRun
	}{
		{
			name:      "empty input gets a zero-values TestRun",
			plz:       &v1alpha1.PrivateLoadZone{},
			cloudData: &cloud.TestRunData{},
			ingestUrl: mainIngest,
			expected:  &defaultTestRun,
		},
		{
			name: "required fields in PLZ",
			plz: &v1alpha1.PrivateLoadZone{
				Spec: v1alpha1.PrivateLoadZoneSpec{
					Token: someToken,
					Resources: corev1.ResourceRequirements{
						Limits: resourceLimits,
					},
				},
			},
			cloudData: &cloud.TestRunData{},
			ingestUrl: mainIngest,
			expected:  &requiredFieldsTestRun,
		},
		{
			name: "optional fields in PLZ",
			plz: &v1alpha1.PrivateLoadZone{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: someNS,
				},
				Spec: v1alpha1.PrivateLoadZoneSpec{
					Token: someToken,
					Resources: corev1.ResourceRequirements{
						Limits: resourceLimits,
					},
					Tolerations:        tolerations,
					ServiceAccountName: someSA,
					NodeSelector:       someNodeSelector,
				},
			},
			cloudData: &cloud.TestRunData{},
			ingestUrl: mainIngest,
			expected:  &optionalFieldsTestRun,
		},
		{
			name: "basic cloud fields",
			plz: &v1alpha1.PrivateLoadZone{
				Spec: v1alpha1.PrivateLoadZoneSpec{
					Token: someToken,
					Resources: corev1.ResourceRequirements{
						Limits: resourceLimits,
					},
				},
			},
			cloudData: &cloud.TestRunData{
				TestRunId: someTestRunID,
				LZConfig: cloud.LZConfig{
					RunnerImage:   someRunnerImage,
					InstanceCount: someInstances,
					ArchiveURL:    someArchiveURL,
				},
			},
			ingestUrl: mainIngest,
			expected:  &cloudFieldsTestRun,
		},
		{
			name: "cloud fields with env vars",
			plz: &v1alpha1.PrivateLoadZone{
				Spec: v1alpha1.PrivateLoadZoneSpec{
					Token: someToken,
					Resources: corev1.ResourceRequirements{
						Limits: resourceLimits,
					},
				},
			},
			cloudData: &cloud.TestRunData{
				TestRunId: someTestRunID,
				LZConfig: cloud.LZConfig{
					RunnerImage:   someRunnerImage,
					InstanceCount: someInstances,
					ArchiveURL:    someArchiveURL,
					Environment:   someEnvVars,
				},
			},
			ingestUrl: mainIngest,
			expected:  &cloudEnvVarsTestRun,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got := NewPLZTestRun(testCase.plz, "token", testCase.cloudData, testCase.ingestUrl)
			if diff := deep.Equal(got, testCase.expected); diff != nil {
				t.Errorf("NewPLZTestRun returned unexpected data, diff: %s", diff)
			}
		})
	}
}
