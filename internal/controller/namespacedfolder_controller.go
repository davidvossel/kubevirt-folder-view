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
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logger "sigs.k8s.io/controller-runtime/pkg/log"

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

func (r *NamespacedFolderReconciler) getAllVMs(ctx context.Context, folder *v1alpha1.NamespacedFolder) ([]string, error) {
	var vms []string
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

		vms = append(vms, vm.Name)
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

func (r *NamespacedFolderReconciler) reconcileFolderPermissions(ctx context.Context, folder *v1alpha1.NamespacedFolder, vms []string) ([]string, []string, error) {

	appliedRoleBindings := []string{}
	appliedRoles := []string{}
	namespace := folder.Namespace

	for _, fp := range folder.Spec.FolderPermissions {
		for _, rr := range fp.RoleRefs {
			name, err := generateRoleBindingNameHash(folder.UID, namespace, fp.Subject, rr)
			if err != nil {
				return appliedRoleBindings, appliedRoles, err
			}

			// TODO, Limit the binding to only the VMs in question.
			// This involves the following
			// 1. look up rolerefs, and make a custom role for each one where only the virtualmachine resources are kept
			// 2. modify each rule in the custom role to only target the named VMs
			// 3. explicitly allow 'list' for all VMs in namespace though, in order for folder UI to work.
			expectedRoleBinding := &rbacv1.RoleBinding{}
			expectedRoleBinding.Name = name
			expectedRoleBinding.Namespace = namespace
			_, err = controllerutil.CreateOrUpdate(ctx, r.Client, expectedRoleBinding, func() error {
				if expectedRoleBinding.Labels == nil {
					expectedRoleBinding.Labels = map[string]string{}
				}
				expectedRoleBinding.Labels[NamespacedFolderOwnershipLabel] = string(folder.UID)

				expectedRoleBinding.Subjects = []rbacv1.Subject{fp.Subject}
				expectedRoleBinding.RoleRef = rr
				return nil
			})
			if err != nil {
				return appliedRoleBindings, appliedRoles, err
			}

			appliedRoleBindings = append(appliedRoleBindings, name)
		}
	}

	return appliedRoleBindings, appliedRoles, nil

}

// +kubebuilder:rbac:groups=kubevirtfolderview.kubevirt.io.github.com,resources=namespacedfolders,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubevirtfolderview.kubevirt.io.github.com,resources=namespacedfolders/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubevirtfolderview.kubevirt.io.github.com,resources=namespacedfolders/finalizers,verbs=update
func (r *NamespacedFolderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	log := logger.FromContext(ctx)

	folder := &v1alpha1.NamespacedFolder{}

	if err := r.Client.Get(ctx, req.NamespacedName, folder); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Get all vms and child folder vms for this folder
	vms, err := r.getAllVMs(ctx, folder)
	if err != nil {
		return ctrl.Result{}, err
	}

	ownerLabels := map[string]string{
		NamespacedFolderOwnershipLabel: string(folder.UID),
	}

	rbList := rbacv1.RoleBindingList{}
	if err := r.Client.List(ctx, &rbList, client.MatchingLabels(ownerLabels)); err != nil {
		return ctrl.Result{}, err
	}

	rList := rbacv1.RoleBindingList{}
	if err := r.Client.List(ctx, &rbList, client.MatchingLabels(ownerLabels)); err != nil {
		return ctrl.Result{}, err
	}

	// Create RoleBindings for this folder in every namespace
	expectedRoleBindings := map[string]bool{}
	expectedRoles := map[string]bool{}

	log.Info(fmt.Sprintf("NAMESPACE: %s\n", folder.Namespace))
	appliedRoleBindings, appliedRoles, err := r.reconcileFolderPermissions(ctx, folder, vms)
	if err != nil {
		return ctrl.Result{}, err
	}

	for _, rbName := range appliedRoleBindings {
		expectedRoleBindings[rbName] = true
	}
	for _, rName := range appliedRoles {
		expectedRoles[rName] = true
	}

	// Cleanup unused rolebindings for this folder
	for _, roleBinding := range rbList.Items {
		_, ok := expectedRoleBindings[roleBinding.Name]
		if !ok {
			err := r.Client.Delete(ctx, &roleBinding)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	}
	// Cleanup unused roles for this folder
	for _, role := range rList.Items {
		_, ok := expectedRoles[role.Name]
		if !ok {
			err := r.Client.Delete(ctx, &role)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
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
