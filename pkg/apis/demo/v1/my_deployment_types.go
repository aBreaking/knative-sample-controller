package v1

import (
	context "context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
)

// MyDeployment
//
// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type MyDeployment struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec MyDeploymentSpec `json:"spec,omitempty"`

	Status MyDeploymentStatus `json:"status,omitempty"`
}

func (m MyDeployment) GetStatus() *duckv1.Status {
	return &m.Status.Status
}

func (m MyDeployment) GetConditionSet() apis.ConditionSet {
	return apis.ConditionSet{}
}

func (m MyDeployment) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("MyDeployment")
}

func (m MyDeployment) SetDefaults(ctx context.Context) {

}

// Validate 可以做一个简单的校验，比如判断传入image内容
func (m MyDeployment) Validate(ctx context.Context) *apis.FieldError {
	fmt.Println("image is ", m.Spec.Image)
	if m.Spec.Image == "" {
		return apis.ErrMissingField("image")
	}
	return nil
}

// MyDeploymentSpec 指定Myployment期望达到什么样的状态
type MyDeploymentSpec struct {
	Image    string `json:"image"`
	Replicas string `json:"replicas"`
}

type MyDeploymentStatus struct {
	duckv1.Status `json:",inline"`
	ReadyReplicas int32 `json:"readyReplicas"`
}

// MyDeploymentList
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type MyDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []MyDeployment `json:"items"`
}

var (
	_ apis.Validatable   = (*MyDeployment)(nil)
	_ apis.Defaultable   = (*MyDeployment)(nil)
	_ kmeta.OwnerRefable = (*MyDeployment)(nil)
	_ duckv1.KRShaped    = (*MyDeployment)(nil)
)
