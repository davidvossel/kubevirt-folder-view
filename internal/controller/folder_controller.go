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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logger "sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/davidvossel/kubevirt-folder-view/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// FolderReconciler reconciles a Folder object
type FolderReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *FolderReconciler) getAllNamespaces(ctx context.Context, folder *v1alpha1.Folder) ([]corev1.Namespace, error) {
	var folderNamespaces []corev1.Namespace
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

	for _, child := range folder.Spec.ChildFolders {
		childFolder := v1alpha1.Folder{}
		name := client.ObjectKey{Name: child}

		err := r.Client.Get(ctx, name, &childFolder)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return folderNamespaces, err
			}
			continue
		}

		childNamespaces, err := r.getAllNamespaces(ctx, &childFolder)
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
func (r *FolderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logger.FromContext(ctx)

	folder := &v1alpha1.Folder{}

	if err := r.Client.Get(ctx, req.NamespacedName, folder); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	//nsList := corev1.NamespaceList{}
	//if err := r.Client.List(ctx, &nsList); err != nil {
	//	return ctrl.Result{}, err
	//}

	//rbList := rbacv1.RoleBindingList{}
	//if err := r.Client.List(ctx, &rbList); err != nil {
	//	return ctrl.Result{}, err
	//}

	folderNamespaces, err := r.getAllNamespaces(ctx, folder)
	if err != nil {
		return ctrl.Result{}, err
	}

	// TODO Folder Logic

	// 1. Collect all namespaces associated with this folder and child folders
	// 2. Collect all existing RB associated with this folder
	//    a. Label on RB pointing back to folder
	//    b. TODO need to make this label immutable
	// 3. generate expected RB for this folder for all namespaces
	//    a. loop through namespaces
	//    b. generate expected rbac objects
	//       - RB name needs to be unique - (use a hash combo feeding in uuid of folder + subject + rb)
	//       - ensure Label is set pointing back to folder
	//       - ensure OwnerReference is accurate
	// 4. apply all expected RB, updating if existing and different, creating if non existent
	// 5. delete all RB that doesn't match an expected RB name list.
	//
	// RB name is unique and meant to be repeatable if subject/rb pairs remain stable. This allows us
	// to determine if a RB should exist or not
	//
	// RB label is meant to be stable and point back to folder

	for _, ns := range folderNamespaces {
		log.Info(fmt.Sprintf("NAMESPACE: %s\n", ns.Name))
	}

	//	for _, rb := range rbList.Items {
	//		fmt.Printf("ROLEBINDING: %s/%s\n", rb.Namespace, rb.Name)
	//	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *FolderReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Folder{}).
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
