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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kubevirtfolderviewkubevirtiov1alpha1 "github.com/davidvossel/kubevirt-folder-view/api/v1alpha1"
)

// nolint:unused
// log is for logging in this package.
var folderindexlog = logf.Log.WithName("folderindex-resource")

// SetupFolderIndexWebhookWithManager registers the webhook for FolderIndex in the manager.
func SetupFolderIndexWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&kubevirtfolderviewkubevirtiov1alpha1.FolderIndex{}).
		WithValidator(&FolderIndexCustomValidator{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-kubevirtfolderview-kubevirt-io-github-com-v1alpha1-folderindex,mutating=false,failurePolicy=fail,sideEffects=None,groups=kubevirtfolderview.kubevirt.io.github.com,resources=folderindices,verbs=create;update,versions=v1alpha1,name=vfolderindex-v1alpha1.kb.io,admissionReviewVersions=v1

// FolderIndexCustomValidator struct is responsible for validating the FolderIndex resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type FolderIndexCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &FolderIndexCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type FolderIndex.
func (v *FolderIndexCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	folderindex, ok := obj.(*kubevirtfolderviewkubevirtiov1alpha1.FolderIndex)
	if !ok {
		return nil, fmt.Errorf("expected a FolderIndex object but got %T", obj)
	}
	folderindexlog.Info("Validation for FolderIndex upon creation", "name", folderindex.GetName())

	// TODO(user): fill in your validation logic upon object creation.

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type FolderIndex.
func (v *FolderIndexCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	folderindex, ok := newObj.(*kubevirtfolderviewkubevirtiov1alpha1.FolderIndex)
	if !ok {
		return nil, fmt.Errorf("expected a FolderIndex object for the newObj but got %T", newObj)
	}
	folderindexlog.Info("Validation for FolderIndex upon update", "name", folderindex.GetName())

	// TODO(user): fill in your validation logic upon object update.

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type FolderIndex.
func (v *FolderIndexCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	folderindex, ok := obj.(*kubevirtfolderviewkubevirtiov1alpha1.FolderIndex)
	if !ok {
		return nil, fmt.Errorf("expected a FolderIndex object but got %T", obj)
	}
	folderindexlog.Info("Validation for FolderIndex upon deletion", "name", folderindex.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}
