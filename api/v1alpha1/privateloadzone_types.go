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
	"context"
	"fmt"

	guuid "github.com/google/uuid"

	"github.com/go-logr/logr"
	"github.com/grafana/k6-operator/pkg/cloud"

	"go.k6.io/k6/cloudapi"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PrivateLoadZoneSpec defines the desired state of PrivateLoadZone
type PrivateLoadZoneSpec struct {
	Token              string                        `json:"token"`
	Resources          corev1.ResourceRequirements   `json:"resources"`
	Tolerations        []corev1.Toleration           `json:"tolerations,omitempty"`
	ServiceAccountName string                        `json:"serviceAccountName,omitempty"`
	NodeSelector       map[string]string             `json:"nodeSelector,omitempty"`
	Image              string                        `json:"image,omitempty"`
	ImagePullSecrets   []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
}

// PrivateLoadZoneStatus defines the observed state of PrivateLoadZone
type PrivateLoadZoneStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// PrivateLoadZone is the Schema for the privateloadzones API
type PrivateLoadZone struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PrivateLoadZoneSpec   `json:"spec,omitempty"`
	Status PrivateLoadZoneStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PrivateLoadZoneList contains a list of PrivateLoadZone
type PrivateLoadZoneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PrivateLoadZone `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PrivateLoadZone{}, &PrivateLoadZoneList{})
}

// Register attempts to register PLZ with the k6 Cloud.
// Regardless of the result, condition PLZRegistered will be set to False.
// Callee is expected to check the returned error and set condition when it's appropriate.
func (plz *PrivateLoadZone) Register(ctx context.Context, logger logr.Logger, client *cloudapi.Client) (string, error) {
	plz.UpdateCondition(PLZRegistered, metav1.ConditionFalse)

	uid := uuid()
	data := cloud.PLZRegistrationData{
		LoadZoneID: plz.Name,
		Resources: cloud.PLZResources{
			CPU:    plz.Spec.Resources.Limits.Cpu().String(),
			Memory: plz.Spec.Resources.Limits.Memory().String(),
		},
		LZConfig: cloud.LZConfig{
			RunnerImage: plz.Spec.Image,
		},
		UID: uid,
	}

	if err := cloud.RegisterPLZ(client, data); err != nil {
		logger.Error(err, fmt.Sprintf("Failed to register PLZ %s.", plz.Name))
		return "", err
	}

	logger.Info(fmt.Sprintf("Registered PLZ %s.", plz.Name))

	return uid, nil
}

// Deregister attempts to deregister PLZ with the k6 Cloud.
// It is meant to be used as a finalizer.
func (plz *PrivateLoadZone) Deregister(ctx context.Context, logger logr.Logger, client *cloudapi.Client) error {
	if err := cloud.DeRegisterPLZ(client, plz.Name); err != nil {
		logger.Error(err, fmt.Sprintf("Failed to de-register PLZ %s.", plz.Name))
		return err
	}

	logger.Info(fmt.Sprintf("De-registered PLZ %s.", plz.Name))

	return nil
}

func uuid() string {
	return guuid.New().String()
}
