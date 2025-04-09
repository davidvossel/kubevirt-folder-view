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

// ClusterFolderPermission defines what roles are applied to a subject
// in order for that subject to have permissions to access the folder
type ClusterFolderPermission struct {
	Subject rbacv1.Subject `json:"subject"`

	RoleRefs []rbacv1.RoleRef `json:"roleRefs,omitempty"`
}

// ClusterFolderSpec defines the desired state of ClusterFolder.
type ClusterFolderSpec struct {
	// +listType=set
	ChildClusterFolders []string `json:"childClusterFolders,omitempty"`

	// +listType=set
	Namespaces []string `json:"namespaces,omitempty"`

	FolderPermissions []ClusterFolderPermission `json:"folderPermissions,omitempty"`
}

// ClusterFolderStatus defines the observed state of ClusterFolder.
type ClusterFolderStatus struct {
	// ParentClusterFolder string `json:"parentClusterFolders,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// ClusterFolder is the Schema for the folders API.
type ClusterFolder struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterFolderSpec   `json:"spec,omitempty"`
	Status ClusterFolderStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterFolderList contains a list of ClusterFolder.
type ClusterFolderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterFolder `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterFolder{}, &ClusterFolderList{})
}
