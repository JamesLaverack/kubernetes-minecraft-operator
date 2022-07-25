package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
	Name string `json:"name,omitempty"`
	UUID string `json:"uuid,omitempty"`
}

type WorldSpec struct {
	PersistentVolumeClaim *corev1.PersistentVolumeClaimVolumeSource `json:"persistentVolumeClaim,omitempty"`
}

type VanillaTweaks struct {
	Survival      []string `json:"survival,omitempty"`
	Items         []string `json:"items,omitempty"`
	Mobs          []string `json:"mobs,omitempty"`
	Teleportation []string `json:"teleportation,omitempty"`
	Utilities     []string `json:"utilities,omitempty"`
	Hermitcraft   []string `json:"hermitcraft,omitempty"`
	Experimental  []string `json:"experimental,omitempty"`
}

type MonitoringSpec struct {
	Enabled bool `json:"enabled"`
}

type DynmapSpec struct {
	Enabled               bool                                      `json:"enabled"`
	PersistentVolumeClaim *corev1.PersistentVolumeClaimVolumeSource `json:"persistentVolumeClaim,omitempty"`
}

// MinecraftServerSpec defines the desired state of MinecraftServer
type MinecraftServerSpec struct {
	EULA              EULAAcceptance  `json:"eula"`
	MinecraftVersion  string          `json:"minecraftVersion"`
	Type              ServerType      `json:"type"`
	AllowList         []Player        `json:"allowList,omitempty"`
	OpsList           []Player        `json:"opsList,omitempty"`
	World             *WorldSpec      `json:"world,omitempty"`
	MOTD              string          `json:"motd"`
	MaxPlayers        int             `json:"maxPlayers"`
	ViewDistance      int             `json:"viewDistance"`
	ExternalServiceIP string          `json:"externalServiceIP"`
	VanillaTweaks     *VanillaTweaks  `json:"vanillaTweaks,omitempty"`
	Monitoring        *MonitoringSpec `json:"monitoring,omitempty"`
	Dynmap            *DynmapSpec     `json:"dynmap,omitempty"`
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
