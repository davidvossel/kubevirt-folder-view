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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logger "sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/davidvossel/kubevirt-folder-view/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

const ClusterFolderOwnershipUIDLabel = "cluster-owner-uid.folderview.kubevirt.io"
const ClusterFolderOwnershipNameLabel = "cluster-owner-name.folderview.kubevirt.io"

// ClusterFolderReconciler reconciles a ClusterFolder object
type ClusterFolderReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func getClusterFolderOwnerReference(folder *v1alpha1.ClusterFolder) *metav1.OwnerReference {
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

func generateRoleBindingNameHash(folderUID types.UID, namespace string, subject rbacv1.Subject, roleRef rbacv1.RoleRef) (string, error) {

	subjectJson, err := json.Marshal(subject)
	if err != nil {
		return "", err
	}
	roleRefJson, err := json.Marshal(roleRef)
	if err != nil {
		return "", err
	}

	s := fmt.Sprintf("%s-%s-%s-%s", string(folderUID), namespace, string(subjectJson), string(roleRefJson))
	h := sha256.New()

	_, err = h.Write([]byte(s))
	if err != nil {
		return "", err
	}

	bs := h.Sum(nil)

	return hex.EncodeToString(bs), nil
}

func (r *ClusterFolderReconciler) reconcileFolderPermissions(ctx context.Context, folder *v1alpha1.ClusterFolder, namespace string) ([]string, error) {
	appliedRBs := []string{}

	ownerRef := getClusterFolderOwnerReference(folder)

	for _, fp := range folder.Spec.FolderPermissions {
		for _, rr := range fp.RoleRefs {
			name, err := generateRoleBindingNameHash(folder.UID, namespace, fp.Subject, rr)
			if err != nil {
				return appliedRBs, err
			}

			expectedRB := &rbacv1.RoleBinding{}
			expectedRB.Name = name
			expectedRB.Namespace = namespace
			_, err = controllerutil.CreateOrUpdate(ctx, r.Client, expectedRB, func() error {
				expectedRB.OwnerReferences = []metav1.OwnerReference{
					*ownerRef,
				}
				if expectedRB.Labels == nil {
					expectedRB.Labels = map[string]string{}
				}
				expectedRB.Labels[ClusterFolderOwnershipUIDLabel] = string(folder.UID)

				expectedRB.Subjects = []rbacv1.Subject{fp.Subject}
				expectedRB.RoleRef = rr
				return nil
			})
			if err != nil {
				return appliedRBs, err
			}

			appliedRBs = append(appliedRBs, name)
		}
	}

	return appliedRBs, nil
}

func (r *ClusterFolderReconciler) clearOwnership(ctx context.Context, oldFolderName string, childNamespace string, childFolderName string) error {

	// Remove references to Namespace from old folder
	oldFolder := &v1alpha1.ClusterFolder{}
	oldFolderNamespacedName := client.ObjectKey{Name: oldFolderName}
	err := r.Client.Get(ctx, oldFolderNamespacedName, oldFolder)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	} else if err != nil && !apierrors.IsNotFound(err) {
		// folder no longer exists, ignore
		return nil
	}

	modified := false
	if childNamespace != "" {
		result := []string{}
		for _, curNS := range oldFolder.Spec.Namespaces {
			if curNS != childNamespace {
				result = append(result, curNS)
			} else {
				modified = true
			}
		}
		oldFolder.Spec.Namespaces = result
	}
	if childFolderName != "" {
		result := []string{}
		for _, cur := range oldFolder.Spec.ChildClusterFolders {
			if cur != childFolderName {
				result = append(result, cur)
			} else {
				modified = true
			}
		}
		oldFolder.Spec.ChildClusterFolders = result
	}

	if modified {
		err := r.Client.Update(ctx, oldFolder)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *ClusterFolderReconciler) claimChildClusterFolders(ctx context.Context, folder *v1alpha1.ClusterFolder) error {
	for _, childName := range folder.Spec.ChildClusterFolders {
		child := &v1alpha1.ClusterFolder{}
		name := client.ObjectKey{Name: childName}
		err := r.Client.Get(ctx, name, child)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
			continue
		}

		needsUpdate := false

		if child.Labels == nil {
			child.Labels = map[string]string{}
		}
		ownerName, exists := child.Labels[ClusterFolderOwnershipNameLabel]

		if !exists {
			needsUpdate = true
		} else if ownerName != folder.Name {
			needsUpdate = true
			err := r.clearOwnership(ctx, ownerName, "", childName)
			if err != nil {
				return err
			}
		}

		if needsUpdate {
			child.Labels[ClusterFolderOwnershipNameLabel] = string(folder.Name)
			err = r.Client.Update(ctx, child)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *ClusterFolderReconciler) claimChildNamespaces(ctx context.Context, folder *v1alpha1.ClusterFolder) error {
	for _, nsName := range folder.Spec.Namespaces {
		ns := &corev1.Namespace{}
		name := client.ObjectKey{Name: nsName}
		err := r.Client.Get(ctx, name, ns)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
			continue
		}

		needsUpdate := false

		if ns.Labels == nil {
			ns.Labels = map[string]string{}
		}
		ownerName, exists := ns.Labels[ClusterFolderOwnershipNameLabel]

		if !exists {
			needsUpdate = true
		} else if ownerName != folder.Name {
			needsUpdate = true
			err := r.clearOwnership(ctx, ownerName, nsName, "")
			if err != nil {
				return err
			}
		}

		if needsUpdate {
			ns.Labels[ClusterFolderOwnershipNameLabel] = string(folder.Name)
			err = r.Client.Update(ctx, ns)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *ClusterFolderReconciler) getAllNamespaces(ctx context.Context, folder *v1alpha1.ClusterFolder, observed map[string]struct{}) ([]corev1.Namespace, bool, error) {
	folderNamespaces := []corev1.Namespace{}
	for _, nsName := range folder.Spec.Namespaces {
		ns := corev1.Namespace{}
		name := client.ObjectKey{Name: nsName}
		err := r.Client.Get(ctx, name, &ns)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return folderNamespaces, false, err
			}
			continue
		}

		observedKey := fmt.Sprintf("namespace/%s", ns.Name)
		_, exists := observed[observedKey]
		if exists {
			return folderNamespaces, true, nil
		}
		observed[observedKey] = struct{}{}

		folderNamespaces = append(folderNamespaces, ns)
	}

	for _, child := range folder.Spec.ChildClusterFolders {
		childClusterFolder := v1alpha1.ClusterFolder{}
		name := client.ObjectKey{Name: child}

		err := r.Client.Get(ctx, name, &childClusterFolder)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return folderNamespaces, false, err
			}
			continue
		}

		observedKey := fmt.Sprintf("clusterfolder/%s", childClusterFolder.Name)
		_, exists := observed[observedKey]
		if exists {
			return folderNamespaces, true, nil
		}
		observed[observedKey] = struct{}{}

		childNamespaces, loopDetected, err := r.getAllNamespaces(ctx, &childClusterFolder, observed)
		if err != nil {
			return folderNamespaces, false, err
		} else if loopDetected {
			return folderNamespaces, true, nil
		}

		folderNamespaces = append(folderNamespaces, childNamespaces...)
	}

	return folderNamespaces, false, nil
}

// +kubebuilder:rbac:groups=kubevirtfolderview.kubevirt.io.github.com,resources=folders,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubevirtfolderview.kubevirt.io.github.com,resources=folders/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubevirtfolderview.kubevirt.io.github.com,resources=folders/finalizers,verbs=update
func (r *ClusterFolderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logger.FromContext(ctx)

	folder := &v1alpha1.ClusterFolder{}

	if err := r.Client.Get(ctx, req.NamespacedName, folder); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if err := r.claimChildClusterFolders(ctx, folder); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.claimChildNamespaces(ctx, folder); err != nil {
		return ctrl.Result{}, err
	}

	// Get all namespaces and child folder namespaces for this folder
	//
	// --- Loop Handling ---
	// If loop is detected, block reconciliation and add requeue. The idea here is that the
	// loop will be resolved once the nested folders are reconciled.
	//
	// TODO, need to handle folders being parents of each other.
	folderNamespaces, loopDetected, err := r.getAllNamespaces(ctx, folder, map[string]struct{}{})
	if err != nil {
		return ctrl.Result{}, err
	} else if loopDetected {
		log.Info(fmt.Sprintf("Loop detected in cluster folder [%s], requeuing", folder.Name))
		return ctrl.Result{RequeueAfter: time.Duration(5 * time.Second)}, nil
	}

	rbList := rbacv1.RoleBindingList{}

	rbLabels := map[string]string{
		ClusterFolderOwnershipUIDLabel: string(folder.UID),
	}
	if err := r.Client.List(ctx, &rbList, client.MatchingLabels(rbLabels)); err != nil {
		return ctrl.Result{}, err
	}

	// Create RoleBindings for this folder in every namespace
	expectedRBs := map[string]bool{}
	for _, ns := range folderNamespaces {
		log.Info(fmt.Sprintf("NAMESPACE: %s\n", ns.Name))
		appliedRBs, err := r.reconcileFolderPermissions(ctx, folder, ns.Name)
		if err != nil {
			return ctrl.Result{}, err
		}

		for _, rbName := range appliedRBs {
			expectedRBs[rbName] = true
		}
	}

	// Cleanup unused rolebindings for this folder
	for _, rb := range rbList.Items {
		_, ok := expectedRBs[rb.Name]
		if !ok {
			err := r.Client.Delete(ctx, &rb)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterFolderReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ClusterFolder{}).
		Named("folder").
		// TODO - reenqueue folder if role or namespace objects change.
		//		Watches(
		//			&corev1.Namespace{},
		//			handler.EnqueueRequestsFromMapFunc(),
		//		).
		//		Watches(
		//			&rbacv1.RoleBinding{},
		//			handler.EnqueueRequestsFromMapFunc(),
		//		).
		Complete(r)

}
