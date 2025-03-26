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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubevirtfolderviewkubevirtiov1alpha1 "github.com/davidvossel/kubevirt-folder-view/api/v1alpha1"
	v1alpha1 "github.com/davidvossel/kubevirt-folder-view/api/v1alpha1"
	virtv1 "kubevirt.io/api/core/v1"
)

const NamespacedFolderOwnershipLabel = "namespaced-owner.folderview.kubevirt.io"

// NamespacedFolderReconciler reconciles a NamespacedFolder object
type NamespacedFolderReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *NamespacedFolderReconciler) getAllVMs(ctx context.Context, folder *v1alpha1.NamespacedFolder) ([]virtv1.VirtualMachine, error) {
	var vms []virtv1.VirtualMachine
	for _, vmName := range folder.Spec.VirtualMachines {
		vm := virtv1.VirtualMachine{}
		name := client.ObjectKey{Name: vmName}
		err := r.Client.Get(ctx, name, &vm)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return vms, err
			}
			continue
		}

		vms = append(vms, vm)
	}

	for _, child := range folder.Spec.ChildNamespacedFolders {
		childNamespacedFolder := v1alpha1.NamespacedFolder{}
		name := client.ObjectKey{Name: child}

		err := r.Client.Get(ctx, name, &childNamespacedFolder)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return vms, err
			}
			continue
		}

		childVMs, err := r.getAllVMs(ctx, &childNamespacedFolder)
		if err != nil {
			return vms, err
		}

		vms = append(vms, childVMs...)
	}

	return vms, nil
}

// +kubebuilder:rbac:groups=kubevirtfolderview.kubevirt.io.github.com,resources=namespacedfolders,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubevirtfolderview.kubevirt.io.github.com,resources=namespacedfolders/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubevirtfolderview.kubevirt.io.github.com,resources=namespacedfolders/finalizers,verbs=update
func (r *NamespacedFolderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	//log := logger.FromContext(ctx)

	folder := &v1alpha1.NamespacedFolder{}

	if err := r.Client.Get(ctx, req.NamespacedName, folder); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NamespacedFolderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubevirtfolderviewkubevirtiov1alpha1.NamespacedFolder{}).
		Named("namespacedfolder").
		// TODO - reenqueue folder if role, rolebindings change
		//		Watches(
		//			&rbacv1.Role{},
		//			handler.EnqueueRequestsFromMapFunc(),
		//		).
		//		Watches(
		//			&rbacv1.RoleBinding{},
		//			handler.EnqueueRequestsFromMapFunc(),
		//		).
		Complete(r)
}
