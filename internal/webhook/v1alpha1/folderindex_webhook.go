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
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	v1alpha1 "github.com/davidvossel/kubevirt-folder-view/api/v1alpha1"
)

// nolint:unused
// log is for logging in this package.
var folderindexlog = logf.Log.WithName("folderindex-resource")

// SetupFolderIndexWebhookWithManager registers the webhook for FolderIndex in the manager.
func SetupFolderIndexWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&v1alpha1.FolderIndex{}).
		WithValidator(&FolderIndexCustomValidator{}).
		Complete()
}

// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-kubevirtfolderview-kubevirt-io-github-com-v1alpha1-folderindex,mutating=false,failurePolicy=fail,sideEffects=None,groups=kubevirtfolderview.kubevirt.io.github.com,resources=folderindices,verbs=create;update,versions=v1alpha1,name=vfolderindex-v1alpha1.kb.io,admissionReviewVersions=v1

// FolderIndexCustomValidator struct is responsible for validating the FolderIndex resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type FolderIndexCustomValidator struct {
}

var _ webhook.CustomValidator = &FolderIndexCustomValidator{}

// Validation rules
//
// duplicates
// 1. a folder cannot be the child of multiple folders
// 2. a namespace cannot be the child of multiple folders
// 3. a VM cannot be the child of multiple folders.
//
// Loops
// 1. a child folder cannot also point to a parent in the same chain.
//

func validateNamespacedEntries(folderIndex *v1alpha1.FolderIndex) error {
	visited := map[string]bool{}
	onPath := map[string]bool{}
	vmParentMap := map[string]string{}
	folderParentMap := map[string]string{}

	var dfs func(folder string) error

	dfs = func(folder string) error {
		if onPath[folder] {
			return fmt.Errorf("folder loop detected. folder [%s] cannot be both a parent and child within the same filesystem hierarchy", folder)
		}
		if visited[folder] {
			return nil
		}

		visited[folder] = true
		onPath[folder] = true

		defer func() { onPath[folder] = false }() // unwind after recursion

		entry, exists := folderIndex.Spec.NamespacedFolderEntries[folder]
		if !exists {
			return nil
		}

		namespace := strings.Split(folder, "/")[0]

		for _, vm := range entry.VirtualMachines {
			vmNamespaceName := fmt.Sprintf("%s/%s", namespace, vm)
			prevParent, exists := vmParentMap[vmNamespaceName]
			if exists {
				return fmt.Errorf("vm [%s] in namespace [%s] is the child of both folder [%s] and folder [%s]", vm, namespace, prevParent, folder)
			}
			vmParentMap[vmNamespaceName] = folder
		}

		for _, child := range entry.ChildFolders {
			prevParent, exists := folderParentMap[child]
			if exists {
				return fmt.Errorf("child folder [%s] is the child of both folder [%s] and folder [%s]", child, prevParent, folder)
			}
			folderParentMap[child] = folder

			if err := dfs(child); err != nil {
				return err
			}
		}
		return nil
	}

	for folder := range folderIndex.Spec.NamespacedFolderEntries {
		if !visited[folder] {
			if err := dfs(folder); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateClusterEntries(folderIndex *v1alpha1.FolderIndex) error {
	visited := map[string]bool{}
	onPath := map[string]bool{}
	namespaceParentMap := map[string]string{}
	folderParentMap := map[string]string{}

	var dfs func(folder string) error

	dfs = func(folder string) error {
		if onPath[folder] {
			return fmt.Errorf("folder loop detected. folder [%s] cannot be both a parent and child within the same filesystem hierarchy", folder)
		}
		if visited[folder] {
			return nil
		}

		visited[folder] = true
		onPath[folder] = true

		defer func() { onPath[folder] = false }() // unwind after recursion

		entry, exists := folderIndex.Spec.ClusterFolderEntries[folder]
		if !exists {
			return nil
		}

		for _, ns := range entry.Namespaces {
			prevParent, exists := namespaceParentMap[ns]
			if exists {
				return fmt.Errorf("namespace [%s] is the child of both folder [%s] and folder [%s]", ns, prevParent, folder)
			}
			namespaceParentMap[ns] = folder
		}

		for _, child := range entry.ChildFolders {
			prevParent, exists := folderParentMap[child]
			if exists {
				return fmt.Errorf("child folder [%s] is the child of both folder [%s] and folder [%s]", child, prevParent, folder)
			}
			folderParentMap[child] = folder

			if err := dfs(child); err != nil {
				return err
			}
		}
		return nil
	}

	for folder := range folderIndex.Spec.ClusterFolderEntries {
		if !visited[folder] {
			if err := dfs(folder); err != nil {
				return err
			}
		}
	}

	return nil
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type FolderIndex.
func (v *FolderIndexCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	folderIndex, ok := obj.(*v1alpha1.FolderIndex)
	if !ok {
		return nil, fmt.Errorf("expected a FolderIndex object but got %T", obj)
	}
	folderindexlog.Info("Validation for FolderIndex upon creation", "name", folderIndex.GetName())

	err := validateClusterEntries(folderIndex)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type FolderIndex.
func (v *FolderIndexCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	folderIndex, ok := newObj.(*v1alpha1.FolderIndex)
	if !ok {
		return nil, fmt.Errorf("expected a FolderIndex object for the newObj but got %T", newObj)
	}
	folderindexlog.Info("Validation for FolderIndex upon update", "name", folderIndex.GetName())

	err := validateClusterEntries(folderIndex)
	if err != nil {
		return nil, err
	}
	err = validateNamespacedEntries(folderIndex)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type FolderIndex.
func (v *FolderIndexCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	folderIndex, ok := obj.(*v1alpha1.FolderIndex)
	if !ok {
		return nil, fmt.Errorf("expected a FolderIndex object but got %T", obj)
	}
	folderindexlog.Info("Validation for FolderIndex upon deletion", "name", folderIndex.GetName())

	return nil, nil
}
