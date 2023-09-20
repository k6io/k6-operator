/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"errors"
	"path/filepath"

	"github.com/grafana/k6-operator/pkg/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PodMetadata struct {
	Annotations map[string]string `json:"annotations,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

type Pod struct {
	Affinity                     *corev1.Affinity              `json:"affinity,omitempty"`
	AutomountServiceAccountToken string                        `json:"automountServiceAccountToken,omitempty"`
	Env                          []corev1.EnvVar               `json:"env,omitempty"`
	Image                        string                        `json:"image,omitempty"`
	ImagePullSecrets             []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	ImagePullPolicy              corev1.PullPolicy             `json:"imagePullPolicy,omitempty"`
	Metadata                     PodMetadata                   `json:"metadata,omitempty"`
	NodeSelector                 map[string]string             `json:"nodeSelector,omitempty"`
	Tolerations                  []corev1.Toleration           `json:"tolerations,omitempty"`
	Resources                    corev1.ResourceRequirements   `json:"resources,omitempty"`
	ServiceAccountName           string                        `json:"serviceAccountName,omitempty"`
	SecurityContext              corev1.PodSecurityContext     `json:"securityContext,omitempty"`
	EnvFrom                      []corev1.EnvFromSource        `json:"envFrom,omitempty"`
	ReadinessProbe               *corev1.Probe                 `json:"readinessProbe,omitempty"`
	LivenessProbe                *corev1.Probe                 `json:"livenessProbe,omitempty"`
	InitContainers               []InitContainer               `json:"initContainers,omitempty"`
	Volumes                      []corev1.Volume               `json:"volumes,omitempty"`
	VolumeMounts                 []corev1.VolumeMount          `json:"volumeMounts,omitempty"`
}

type InitContainer struct {
	Name         string                 `json:"name,omitempty"`
	Image        string                 `json:"image,omitempty"`
	Env          []corev1.EnvVar        `json:"env,omitempty"`
	EnvFrom      []corev1.EnvFromSource `json:"envFrom,omitempty"`
	Command      []string               `json:"command,omitempty"`
	Args         []string               `json:"args,omitempty"`
	WorkingDir   string                 `json:"workingDir,omitempty"`
	VolumeMounts []corev1.VolumeMount   `json:"volumeMounts,omitempty"`
}

type K6Scuttle struct {
	Enabled                 string `json:"enabled,omitempty"`
	EnvoyAdminApi           string `json:"envoyAdminApi,omitempty"`
	NeverKillIstio          bool   `json:"neverKillIstio,omitempty"`
	NeverKillIstioOnFailure bool   `json:"neverKillIstioOnFailure,omitempty"`
	DisableLogging          bool   `json:"disableLogging,omitempty"`
	StartWithoutEnvoy       bool   `json:"startWithoutEnvoy,omitempty"`
	WaitForEnvoyTimeout     string `json:"waitForEnvoyTimeout,omitempty"`
	IstioQuitApi            string `json:"istioQuitApi,omitempty"`
	GenericQuitEndpoint     string `json:"genericQuitEndpoint,omitempty"`
	QuitWithoutEnvoyTimeout string `json:"quitWithoutEnvoyTimeout,omitempty"`
}

// K6Spec defines the desired state of K6
type K6Spec struct {
	Script      K6Script               `json:"script"`
	Parallelism int32                  `json:"parallelism"`
	Separate    bool                   `json:"separate,omitempty"`
	Arguments   string                 `json:"arguments,omitempty"`
	Ports       []corev1.ContainerPort `json:"ports,omitempty"`
	Initializer *Pod                   `json:"initializer,omitempty"`
	Starter     Pod                    `json:"starter,omitempty"`
	Runner      Pod                    `json:"runner,omitempty"`
	Quiet       string                 `json:"quiet,omitempty"`
	Paused      string                 `json:"paused,omitempty"`
	Scuttle     K6Scuttle              `json:"scuttle,omitempty"`
	Cleanup     Cleanup                `json:"cleanup,omitempty"`

	TestRunID string `json:"testRunId,omitempty"` // PLZ reserved field
	Token     string `json:"token,omitempty"`     // PLZ reserved field (for now)
}

// K6Script describes where the script to execute the tests is found
type K6Script struct {
	VolumeClaim K6VolumeClaim `json:"volumeClaim,omitempty"`
	ConfigMap   K6Configmap   `json:"configMap,omitempty"`
	LocalFile   string        `json:"localFile,omitempty"`
}

// K6VolumeClaim describes the volume claim script location
type K6VolumeClaim struct {
	Name string `json:"name"`
	File string `json:"file,omitempty"`
}

// K6Configmap describes the config map script location
type K6Configmap struct {
	Name string `json:"name"`
	File string `json:"file,omitempty"`
}

//TODO: cleanup pre-execution?

// Cleanup allows for automatic cleanup of resources post execution
// +kubebuilder:validation:Enum=post;whenOk;whenFailed
type Cleanup string

// Stage describes which stage of the test execution lifecycle our runners are in
// +kubebuilder:validation:Enum=initialization;initialized;created;started;stopped;finished;error
type Stage string

// K6Status defines the observed state of K6
type K6Status struct {
	Stage           Stage  `json:"stage,omitempty"`
	TestRunID       string `json:"testRunId,omitempty"`
	AggregationVars string `json:"aggregationVars,omitempty"`

	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// K6 is the Schema for the k6s API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Stage",type="string",JSONPath=".status.stage",description="Stage"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="TestRunID",type="string",JSONPath=".status.testRunId"
type K6 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   K6Spec   `json:"spec,omitempty"`
	Status K6Status `json:"status,omitempty"`
}

// K6List contains a list of K6
// +kubebuilder:object:root=true
type K6List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []K6 `json:"items"`
}

func init() {
	SchemeBuilder.Register(&K6{}, &K6List{})
}

// Parse extracts Script data bits from K6 spec and performs basic validation
func (k6 K6Spec) ParseScript() (*types.Script, error) {
	spec := k6.Script
	s := &types.Script{
		Filename: "test.js",
		Path:     "/test/",
	}

	if spec.VolumeClaim.Name != "" {
		s.Name = spec.VolumeClaim.Name
		if spec.VolumeClaim.File != "" {
			s.Filename = spec.VolumeClaim.File
		}

		s.Type = "VolumeClaim"
		return s, nil
	}

	if spec.ConfigMap.Name != "" {
		s.Name = spec.ConfigMap.Name

		if spec.ConfigMap.File != "" {
			s.Filename = spec.ConfigMap.File
		}

		s.Type = "ConfigMap"
		return s, nil
	}

	if spec.LocalFile != "" {
		s.Name = "LocalFile"
		s.Type = "LocalFile"
		s.Path, s.Filename = filepath.Split(spec.LocalFile)
		return s, nil
	}

	return nil, errors.New("Script definition should contain one of: ConfigMap, VolumeClaim, LocalFile")
}
