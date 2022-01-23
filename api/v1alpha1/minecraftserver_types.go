/*
Copyright 2022.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +kubebuilder:validation:Enum=Paper;Forge
type ServerType string

const (
	ServerTypePaper ServerType = "Paper"
	ServerTypeForge ServerType = "Forge"
)

// +kubebuilder:validation:Enum=Accepted;NotAccepted
type EULAAcceptance string

const (
	EULAAcceptanceAccepted    EULAAcceptance = "Accepted"
	EULAAcceptanceNotAccepted EULAAcceptance = "NotAccepted"
)

// Player is a Minecraft player defined by a username or a UUID
type Player struct {
	Username string `json:"username,omitempty"`
	UUID     string `json:"uuid,omitempty"`
}

type WorldSpec struct {
	PersistentVolumeClaim *corev1.PersistentVolumeClaimVolumeSource `json:"persistentVolumeClaim,omitempty"`
}

// MinecraftServerSpec defines the desired state of MinecraftServer
type MinecraftServerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	EULA             EULAAcceptance               `json:"eula"`
	MinecraftVersion string                       `json:"minecraftVersion"`
	Type             ServerType                   `json:"type"`
	AllowList        []Player                     `json:"allowList,omitempty"`
	World            *WorldSpec                   `json:"world,omitempty"`
	MOTD             string                       `json:"motd"`
	MaxPlayers       int                          `json:"maxPlayers"`
	ViewDistance     int                          `json:"viewDistance"`
}

// MinecraftServerStatus defines the observed state of MinecraftServer
type MinecraftServerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.spec.minecraftVersion`
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// MinecraftServer is the Schema for the minecraftservers API
type MinecraftServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MinecraftServerSpec   `json:"spec,omitempty"`
	Status MinecraftServerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MinecraftServerList contains a list of MinecraftServer
type MinecraftServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MinecraftServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MinecraftServer{}, &MinecraftServerList{})
}
