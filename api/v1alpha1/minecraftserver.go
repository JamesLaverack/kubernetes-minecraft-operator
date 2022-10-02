package v1alpha1

//go:generate controller-gen crd output:crd:artifacts:config=../../deploy/crd
//go:generate controller-gen object

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
	Overworld *corev1.PersistentVolumeClaimVolumeSource `json:"overworld,omitempty"`
	Nether    *corev1.PersistentVolumeClaimVolumeSource `json:"nether,omitempty"`
	TheEnd    *corev1.PersistentVolumeClaimVolumeSource `json:"theEnd,omitempty"`
}

type VanillaTweaks struct {
	Datapacks []VanillaTweaksDatapack `json:"datapacks,omitempty"`
}

type VanillaTweaksDatapack struct {
	Name     string `json:"name"`
	Category string `json:"category"`
}

// +kubebuilder:validation:Enum=Disabled;PrometheusServiceMonitor
type MonitoringType string

type MonitoringSpec struct {
	Type MonitoringType `json:"type"`
}

const MonitoringTypeDisabled MonitoringType = "Disabled"
const MonitoringTypePrometheusServiceMonitor MonitoringType = "PrometheusServiceMonitor"

type DynmapSpec struct {
	Enabled    bool                                      `json:"enabled"`
	MapStorage *corev1.PersistentVolumeClaimVolumeSource `json:"persistentVolumeClaim,omitempty"`
}

// +kubebuilder:validation:Enum=Survival;Creative
// +kubebuilder:default:=Survial
type GameMode string

const GameModeSurvival GameMode = "Survival"
const GameModeCreative GameMode = "Creative"

// +kubebuilder:validation:Enum=Public;AllowListOnly
// +kubebuilder:default:=AllowListOnly
type AccessMode string

const AccessModeAllowListOnly AccessMode = "AllowListOnly"
const AccessModePublic AccessMode = "Public"

// MinecraftServerSpec defines the desired state of MinecraftServer
type MinecraftServerSpec struct {
	EULA             EULAAcceptance  `json:"eula"`
	MinecraftVersion string          `json:"minecraftVersion"`
	Type             ServerType      `json:"type"`
	AccessMode       AccessMode      `json:"accessMode"`
	AllowList        []Player        `json:"allowList,omitempty"`
	OpsList          []Player        `json:"opsList,omitempty"`
	World            *WorldSpec      `json:"world,omitempty"`
	MOTD             string          `json:"motd"`
	GameMode         GameMode        `json:"gameMode"`
	MaxPlayers       int             `json:"maxPlayers"`
	ViewDistance     int             `json:"viewDistance"`
	Service          *ServiceSpec    `json:"service"`
	VanillaTweaks    *VanillaTweaks  `json:"vanillaTweaks,omitempty"`
	Monitoring       *MonitoringSpec `json:"monitoring,omitempty"`
	Dynmap           *DynmapSpec     `json:"dynmap,omitempty"`
}

// +kubebuilder:validation:Enum=None;ClusterIP;NodePort;LoadBalancer
type ServiceType string

const ServiceTypeNone ServiceType = "None"
const ServiceTypeClusterIP ServiceType = "ClusterIP"
const ServiceTypeNodePort ServiceType = "NodePort"
const ServiceTypeLoadBalancer ServiceType = "LoadBalancer"

// ServiceSpec is very much like a corev1.ServiceSpec, but with only *some* fields.
type ServiceSpec struct {
	Type ServiceType `json:"type"`
	// Port to bind Minecraft to if using a NodePort or LoadBalancer service
	MinecraftNodePort *int32 `json:"minecraftNodePort,omitempty"`
}

// +kubebuilder:validation:Enum=Pending;Running;Error
type State string

const StatePending State = "Pending"
const StateRunning State = "Running"
const StateError State = "Error"

// MinecraftServerStatus defines the observed state of MinecraftServer
type MinecraftServerStatus struct {
	State State `json:"state"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.spec.minecraftVersion`
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
