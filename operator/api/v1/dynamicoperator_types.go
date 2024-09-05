package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DynamicOperatorSpec defines the desired state of DynamicOperator
// +kubebuilder:validation:XValidation:rule="self.onCreate != \"\" || self.onUpdate != \"\" || self.onDelete != \"\"",message="At least one of onCreate, onUpdate, or onDelete must be specified"
type DynamicOperatorSpec struct {
	// ResourceKind specifies the Kubernetes resource kind to watch
	// +kubebuilder:validation:Required
	ResourceKind string `json:"resourceKind"`

	// Namespace specifies the namespace to watch. If empty, it watches all namespaces
	Namespace string `json:"namespace,omitempty"`

	// OnCreate is the Cyphernetes query to execute when a resource is created
	OnCreate string `json:"onCreate,omitempty"`

	// OnUpdate is the Cyphernetes query to execute when a resource is updated
	OnUpdate string `json:"onUpdate,omitempty"`

	// OnDelete is the Cyphernetes query to execute when a resource is deleted
	OnDelete string `json:"onDelete,omitempty"`

	// Finalizer specifies whether the operator should register itself as a finalizer on the watched resources
	Finalizer bool `json:"finalizer,omitempty"`
}

// DynamicOperatorStatus defines the observed state of DynamicOperator
type DynamicOperatorStatus struct {
	// ActiveWatchers is the number of active watchers for this DynamicOperator
	ActiveWatchers int `json:"activeWatchers"`

	// LastExecutedQuery is the last Cyphernetes query that was executed
	LastExecutedQuery string `json:"lastExecutedQuery,omitempty"`

	// LastExecutionTime is the timestamp of the last query execution
	LastExecutionTime *metav1.Time `json:"lastExecutionTime,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="ResourceKind",type=string,JSONPath=`.spec.resourceKind`
//+kubebuilder:printcolumn:name="Namespace",type=string,JSONPath=`.spec.namespace`
//+kubebuilder:printcolumn:name="ActiveWatchers",type=integer,JSONPath=`.status.activeWatchers`

// DynamicOperator is the Schema for the dynamicoperators API
type DynamicOperator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DynamicOperatorSpec   `json:"spec,omitempty"`
	Status DynamicOperatorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DynamicOperatorList contains a list of DynamicOperator
type DynamicOperatorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DynamicOperator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DynamicOperator{}, &DynamicOperatorList{})
}
