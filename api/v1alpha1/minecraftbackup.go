package v1alpha1

//go:generate controller-gen crd output:crd:artifacts:config=../../crd
//go:generate controller-gen object

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MinecraftServerLocator struct {
	Name string `json:"name"`
}

type MinecraftBackupSpec struct {
	Server            MinecraftServerLocator                    `json:"server"`
	BackupDestination *corev1.PersistentVolumeClaimVolumeSource `json:"backupDestination,omitempty"`
}

// +kubebuilder:validation:Enum:=Pending;Failed;Complete
// +kubebuilder:default:=Pending
type BackupState string

const (
	BackupStatePending  BackupState = "Pending"
	BackupStateComplete BackupState = "Complete"
	BackupStateFailed   BackupState = "Failed"
)

type MinecraftBackupStatus struct {
	State BackupState `json:"state"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Server",type=string,JSONPath=`.spec.server.name`
type MinecraftBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MinecraftBackupSpec   `json:"spec,omitempty"`
	Status MinecraftBackupStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type MinecraftBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MinecraftBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MinecraftBackup{}, &MinecraftBackupList{})
}
