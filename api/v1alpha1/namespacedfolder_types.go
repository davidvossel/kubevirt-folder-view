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

// NamespacedFolderSpec defines the desired state of NamespacedFolder.
type NamespacedFolderSpec struct {
	ChildNamespacedFolders []string                  `json:"childNamespacedFolders,omitempty"`
	FolderPermissions      []ClusterFolderPermission `json:"folderPermissions,omitempty"`
	VirtualMachines        []string                  `json:"virtualMachines,omitempty"`
}

// NamespacedFolderStatus defines the observed state of NamespacedFolder.
type NamespacedFolderStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// NamespacedFolder is the Schema for the namespacedfolders API.
type NamespacedFolder struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NamespacedFolderSpec   `json:"spec,omitempty"`
	Status NamespacedFolderStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NamespacedFolderList contains a list of NamespacedFolder.
type NamespacedFolderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NamespacedFolder `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NamespacedFolder{}, &NamespacedFolderList{})
}
