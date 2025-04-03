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

const ClusterFolderOwnershipLabel = "cluster-owner.folderview.kubevirt.io"

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
				expectedRB.Labels[ClusterFolderOwnershipLabel] = string(folder.UID)

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

func (r *ClusterFolderReconciler) getAllNamespaces(ctx context.Context, folder *v1alpha1.ClusterFolder) ([]corev1.Namespace, error) {
	folderNamespaces := []corev1.Namespace{}
	for _, nsName := range folder.Spec.Namespaces {
		ns := corev1.Namespace{}
		name := client.ObjectKey{Name: nsName}
		err := r.Client.Get(ctx, name, &ns)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return folderNamespaces, err
			}
			continue
		}

		folderNamespaces = append(folderNamespaces, ns)
	}

	for _, child := range folder.Spec.ChildClusterFolders {
		childClusterFolder := v1alpha1.ClusterFolder{}
		name := client.ObjectKey{Name: child}

		err := r.Client.Get(ctx, name, &childClusterFolder)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return folderNamespaces, err
			}
			continue
		}

		childNamespaces, err := r.getAllNamespaces(ctx, &childClusterFolder)
		if err != nil {
			return folderNamespaces, err
		}

		folderNamespaces = append(folderNamespaces, childNamespaces...)
	}

	return folderNamespaces, nil
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

	// TODO handle nested folder loops and identical namespace entries across multiple folders
	// Get all namespaces and child folder namespaces for this folder
	folderNamespaces, err := r.getAllNamespaces(ctx, folder)
	if err != nil {
		return ctrl.Result{}, err
	}

	rbList := rbacv1.RoleBindingList{}

	rbLabels := map[string]string{
		ClusterFolderOwnershipLabel: string(folder.UID),
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
