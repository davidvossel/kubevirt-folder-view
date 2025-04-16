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
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logger "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

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
const ClusterFolderOwnershipClaimTimestampLabel = "cluster-owner-claim-timestamp.folderview.kubevirt.io"

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

	// TODO Verify Namespace exists before adding RoleBindings

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

func (r *ClusterFolderReconciler) getAllNamespaces(root *v1alpha1.FolderIndex, folderName string) ([]string, error) {

	folderNamespaces := []string{}

	entry, exists := root.Spec.ClusterFolderEntries[folderName]

	// Folder does not exist
	// TODO garbage collect non existent folders
	if !exists {
		return folderNamespaces, nil
	}

	folderNamespaces = append(folderNamespaces, entry.Namespaces...)

	for _, childFolderName := range entry.ChildFolders {
		var err error
		folderNamespaces, err = r.getAllNamespaces(root, childFolderName)
		if err != nil {
			return folderNamespaces, err
		}
	}

	return folderNamespaces, nil
}

// +kubebuilder:rbac:groups=kubevirtfolderview.kubevirt.io.github.com,resources=folders,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubevirtfolderview.kubevirt.io.github.com,resources=folders/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubevirtfolderview.kubevirt.io.github.com,resources=folders/finalizers,verbs=update
func (r *ClusterFolderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logger.FromContext(ctx)

	root := &v1alpha1.FolderIndex{}

	folder := &v1alpha1.ClusterFolder{}

	log.Info(fmt.Sprintf("Reconciling cluster folder [%s]", req.NamespacedName.Name))
	if err := r.Client.Get(ctx, req.NamespacedName, folder); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if err := r.Client.Get(ctx, client.ObjectKey{Name: "root"}, root); err != nil {
		return ctrl.Result{}, err
	}

	// Get all namespaces and child folder namespaces for this folder
	folderNamespaces, err := r.getAllNamespaces(root, folder.Name)
	if err != nil {
		return ctrl.Result{}, err
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
		appliedRBs, err := r.reconcileFolderPermissions(ctx, folder, ns)
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

func (r *ClusterFolderReconciler) namespaceWatchHandler(ctx context.Context, resource client.Object) []reconcile.Request {
	name := resource.GetName()
	labels := resource.GetLabels()

	_, exists := labels[ClusterFolderOwnershipNameLabel]
	if exists {
		return nil
	}

	log := logger.FromContext(ctx)

	folders := v1alpha1.ClusterFolderList{}
	if err := r.Client.List(ctx, &folders); err != nil {
		return nil
	}

	requests := []reconcile.Request{}
	for _, parent := range folders.Items {
		for _, child := range parent.Spec.Namespaces {
			if child == name {
				log.Info(fmt.Sprintf("queueing parent folder [%s] that has not claimed newly discovered namespace [%s]", parent.Name, name))
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: parent.Name}})
			}
		}
	}

	return requests
}

func (r *ClusterFolderReconciler) folderWatchHandler(ctx context.Context, resource client.Object) []reconcile.Request {
	name := resource.GetName()
	labels := resource.GetLabels()

	_, exists := labels[ClusterFolderOwnershipNameLabel]
	if exists {
		return nil
	}

	log := logger.FromContext(ctx)

	folders := v1alpha1.ClusterFolderList{}
	if err := r.Client.List(ctx, &folders); err != nil {
		return nil
	}

	requests := []reconcile.Request{}
	for _, parent := range folders.Items {
		for _, child := range parent.Spec.ChildClusterFolders {
			if child == name {

				log.Info(fmt.Sprintf("queueing parent folder [%s] that has not claimed newly discovered child [%s]", parent.Name, name))
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: parent.Name}})
			}
		}
	}

	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterFolderReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ClusterFolder{}).
		Named("folder").
		Watches(
			&corev1.Namespace{},
			handler.EnqueueRequestsFromMapFunc(r.namespaceWatchHandler),
		).
		Watches(
			&v1alpha1.ClusterFolder{},
			handler.EnqueueRequestsFromMapFunc(r.folderWatchHandler),
		).
		//		Watches(
		//			&rbacv1.RoleBinding{},
		//			handler.EnqueueRequestsFromMapFunc(),
		//		).
		Complete(r)

}
