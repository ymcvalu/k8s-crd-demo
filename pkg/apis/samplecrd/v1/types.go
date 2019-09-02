package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//
// +genclient
// --+genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type NFSVolume struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Status VolumeStatus `json:"status,omitempty"`

	Spec NFSVolumeSpec `json:"spec"`
}

type VolumeStatus struct {
	Phase string `json:"phase"`
	Msg   string `json:"msg"`
}

type NFSVolumeSpec struct {
	Path       string           `json:"path"` // nfs path
	AccessMode VolumeAccessMode `json:"access_mode"`
}

type VolumeAccessMode string

const (
	ReadWriteOnce VolumeAccessMode = "ReadWriteOnce"
	ReadOnlyMany  VolumeAccessMode = "ReadOnlyMany"
	ReadWriteMany VolumeAccessMode = "ReadWriteMany"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NFSVolumeList is a list of NFSVolume resources
type NFSVolumeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []NFSVolumeSpec `json:"items"`
}
