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
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FolderPermission defines what roles are applied to a subject
// in order for that subject to have permissions to access the folder
type FolderPermission struct {
	Subject rbacv1.Subject `json:"subject"`

	RoleRefs []rbacv1.RoleRef `json:"rolerefs,omitempty"`
}

// FolderSpec defines the desired state of Folder.
type FolderSpec struct {
	ChildFolders      []string           `json:"childFolders,omitempty"`
	FolderPermissions []FolderPermission `json:"folderPermissions,omitempty"`
	Namespaces        []string           `json:"namespaces,omitempty"`
}

// FolderStatus defines the observed state of Folder.
type FolderStatus struct {
	//ParentFolder string `json:"parentFolders,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// Folder is the Schema for the folders API.
type Folder struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FolderSpec   `json:"spec,omitempty"`
	Status FolderStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FolderList contains a list of Folder.
type FolderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Folder `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Folder{}, &FolderList{})
}
