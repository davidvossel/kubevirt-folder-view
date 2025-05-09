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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logger "sigs.k8s.io/controller-runtime/pkg/log"

	kubevirtfolderviewkubevirtiov1alpha1 "github.com/davidvossel/kubevirt-folder-view/api/v1alpha1"
	v1alpha1 "github.com/davidvossel/kubevirt-folder-view/api/v1alpha1"
)

const NamespacedFolderOwnershipLabel = "namespaced-owner.folderview.kubevirt.io"

// NamespacedFolderReconciler reconciles a NamespacedFolder object
type NamespacedFolderReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func getNamespacedFolderOwnerReference(folder *v1alpha1.NamespacedFolder) *metav1.OwnerReference {
	t := true
	return &metav1.OwnerReference{
		APIVersion:         folder.APIVersion,
		Kind:               folder.Kind,
		Name:               folder.Name,
		UID:                folder.UID,
		Controller:         &t,
		BlockOwnerDeletion: &t,
	}
}

func (r *NamespacedFolderReconciler) getAllVMs(root *v1alpha1.FolderIndex, folderName string) ([]string, error) {
	vms := []string{}

	entry, exists := root.Spec.NamespacedFolderEntries[folderName]
	// Folder does not exist
	if !exists {
		return vms, nil
	}

	vms = append(vms, entry.VirtualMachines...)

	for _, childFolderName := range entry.ChildFolders {
		var err error
		vms, err = r.getAllVMs(root, childFolderName)
		if err != nil {
			return vms, err
		}
	}

	return vms, nil
}

func generateRoleNameHash(folderUID types.UID, namespace string, rules []rbacv1.PolicyRule) (string, error) {

	rulesJson, err := json.Marshal(rules)
	if err != nil {
		return "", err
	}

	s := fmt.Sprintf("%s-%s-%s", string(folderUID), namespace, string(rulesJson))
	h := sha256.New()

	_, err = h.Write([]byte(s))
	if err != nil {
		return "", err
	}

	bs := h.Sum(nil)

	return hex.EncodeToString(bs), nil
}

func (r *NamespacedFolderReconciler) reconcileRole(ctx context.Context, folder *v1alpha1.NamespacedFolder, roleRef *rbacv1.RoleRef, vms []string) (string, error) {

	var rules []rbacv1.PolicyRule

	ownerRef := getNamespacedFolderOwnerReference(folder)

	if roleRef.Kind == "Role" {
		role := &rbacv1.Role{}
		name := client.ObjectKey{Name: roleRef.Name, Namespace: folder.Namespace}

		if err := r.Client.Get(ctx, name, role); err != nil {
			if apierrors.IsNotFound(err) {
				return "", nil
			}
			return "", err
		}

		rules = role.Rules
	} else if roleRef.Kind == "ClusterRole" {
		clusterRole := &rbacv1.ClusterRole{}
		name := client.ObjectKey{Name: roleRef.Name}

		if err := r.Client.Get(ctx, name, clusterRole); err != nil {
			if apierrors.IsNotFound(err) {
				return "", nil
			}
			return "", err
		}

		rules = clusterRole.Rules
	} else {
		// unknown kind, ignore
		return "", nil
	}

	// filter rules and add resource names to them
	newRules := []rbacv1.PolicyRule{}

	for _, rule := range rules {
		foundGroups := []string{}

		if len(rule.ResourceNames) != 0 {
			// Ignoring rules with specific resource names right now.
			// TODO make this compatible with roles with ResourceNames by
			// merging resourceNames with vm names
			continue
		}
		for _, group := range rule.APIGroups {
			if group == "kubevirt.io" || group == "subresources.kubevirt.io" {
				foundGroups = append(foundGroups, group)
			}
		}

		foundResources := []string{}
		for _, resource := range rule.Resources {
			baseResource := strings.Split(resource, "/")[0]
			if resource == "*" ||
				baseResource == "virtualmachineinstances" ||
				baseResource == "virtualmachines" {

				foundResources = append(foundResources, resource)
			}
		}

		if len(foundGroups) == 0 || len(foundResources) == 0 {
			continue
		}

		newRule := rbacv1.PolicyRule{
			Verbs:         rule.Verbs,
			APIGroups:     foundGroups,
			Resources:     foundResources,
			ResourceNames: vms,
		}

		newRules = append(newRules, newRule)

	}

	if len(newRules) == 0 {
		return "", nil
	}

	roleName, err := generateRoleNameHash(folder.UID, folder.Namespace, newRules)

	if err != nil {
		return "", err
	}

	newRole := &rbacv1.Role{}
	newRole.Name = roleName
	newRole.Namespace = folder.Namespace

	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, newRole, func() error {
		newRole.OwnerReferences = []metav1.OwnerReference{
			*ownerRef,
		}
		if newRole.Labels == nil {
			newRole.Labels = map[string]string{}
		}
		newRole.Labels[NamespacedFolderOwnershipLabel] = string(folder.UID)

		newRole.Rules = newRules
		return nil
	})
	if err != nil {
		return "", err
	}

	return roleName, nil

}

func (r *NamespacedFolderReconciler) reconcileFolderPermissions(ctx context.Context, folder *v1alpha1.NamespacedFolder, vms []string) ([]string, []string, error) {

	appliedRoleBindings := []string{}
	appliedRoles := []string{}
	namespace := folder.Namespace

	ownerRef := getNamespacedFolderOwnerReference(folder)
	for _, fp := range folder.Spec.FolderPermissions {
		for _, existingRR := range fp.RoleRefs {

			roleName, err := r.reconcileRole(ctx, folder, &existingRR, vms)

			if err != nil {
				return appliedRoleBindings, appliedRoles, err
			} else if roleName == "" {
				// role isn't related to virtual machines
				continue
			}

			appliedRoles = append(appliedRoles, roleName)
			rr := rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     roleName,
			}

			name, err := generateRoleBindingNameHash(folder.UID, namespace, fp.Subject, rr)
			if err != nil {
				return appliedRoleBindings, appliedRoles, err
			}

			expectedRoleBinding := &rbacv1.RoleBinding{}
			expectedRoleBinding.Name = name
			expectedRoleBinding.Namespace = namespace
			_, err = controllerutil.CreateOrUpdate(ctx, r.Client, expectedRoleBinding, func() error {
				expectedRoleBinding.OwnerReferences = []metav1.OwnerReference{
					*ownerRef,
				}
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

	root := &v1alpha1.FolderIndex{}
	folder := &v1alpha1.NamespacedFolder{}

	log.Info(fmt.Sprintf("Reconciling namespaced folder [%s]", req.NamespacedName.Name))
	if err := r.Client.Get(ctx, req.NamespacedName, folder); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// TODO enforce that only a single folder index named root can exist
	if err := r.Client.Get(ctx, client.ObjectKey{Name: "root"}, root); err != nil {
		return ctrl.Result{}, err
	}

	// Get all vms and child folder vms for this folder
	vms, err := r.getAllVMs(root, folder.Name)
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

	rList := rbacv1.RoleList{}
	if err := r.Client.List(ctx, &rList, client.MatchingLabels(ownerLabels)); err != nil {
		return ctrl.Result{}, err
	}

	// Create RoleBindings for this folder in every namespace
	expectedRoleBindings := map[string]bool{}
	expectedRoles := map[string]bool{}

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
			log.Info(fmt.Sprintf("Deleted unused roleBinding: %s\n", roleBinding.Name))
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
			log.Info(fmt.Sprintf("Deleted unused role: %s\n", role.Name))
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
