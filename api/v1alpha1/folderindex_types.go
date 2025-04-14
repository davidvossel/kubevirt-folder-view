/*
Copyright 2025.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NamespacedFolderEntry struct {
	FolderName              string                  `json:"name"`
	NamespacedFolderEntries []NamespacedFolderEntry `json:"namespacedFolderEntries,omitempty"`
	VirtualMachines         []string                `json:"virtualMachines,omitempty"`
}

type NamespaceEntry struct {
	Namespace               string                  `json:"namespace"`
	NamespacedFolderEntries []NamespacedFolderEntry `json:"namespacedFolderEntries,omitempty"`
}

type ClusterFolderEntry struct {
	FolderName           string               `json:"name"`
	ClusterFolderEntries []ClusterFolderEntry `json:"clusterFolderEntries,omitempty"`
	NamespaceEntries     []NamespaceEntry     `json:"namespaceEntries,omitempty"`
}

// FolderIndexSpec defines the desired state of FolderIndex.
type FolderIndexSpec struct {
	ClusterFolderEntries []ClusterFolderEntry `json:"clusterFolderEntries,omitempty"`
	NamespaceEntries     []NamespaceEntry     `json:"namespaceEntries,omitempty"`
}

// FolderIndexStatus defines the observed state of FolderIndex.
type FolderIndexStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// FolderIndex is the Schema for the folderindices API.
type FolderIndex struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FolderIndexSpec   `json:"spec,omitempty"`
	Status FolderIndexStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FolderIndexList contains a list of FolderIndex.
type FolderIndexList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FolderIndex `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FolderIndex{}, &FolderIndexList{})
}
