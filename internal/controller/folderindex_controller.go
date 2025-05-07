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

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kubevirtfolderviewkubevirtiov1alpha1 "github.com/davidvossel/kubevirt-folder-view/api/v1alpha1"
)

// FolderIndexReconciler reconciles a FolderIndex object
type FolderIndexReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kubevirtfolderview.kubevirt.io.github.com,resources=*,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubevirtfolderview.kubevirt.io.github.com,resources=folderindices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubevirtfolderview.kubevirt.io.github.com,resources=folderindices/finalizers,verbs=update

func (r *FolderIndexReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO - The index reconciler should do the following
	// 1. Verify consistency of the root index (same as validating webhook)
	// 2. report status if consistency errors occur
	// 3. set hash of spec that was last validated, this lets the other controllers only reconcile a root
	// index that has been validated (avoiding loops and consistency errors)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *FolderIndexReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubevirtfolderviewkubevirtiov1alpha1.FolderIndex{}).
		Named("folderindex").
		Complete(r)
}
