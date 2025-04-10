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
	"strconv"
	"time"

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

		_, exists = child.Labels[ClusterFolderOwnershipClaimTimestampLabel]
		if !exists {
			needsUpdate = true
		}

		if needsUpdate {
			child.Labels[ClusterFolderOwnershipNameLabel] = string(folder.Name)
			child.Labels[ClusterFolderOwnershipClaimTimestampLabel] = fmt.Sprintf("%d", time.Now().UTC().Unix())
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

func (r *ClusterFolderReconciler) loopChainWinner(ctx context.Context, loopedFolder *v1alpha1.ClusterFolder, curFolder *v1alpha1.ClusterFolder) (*v1alpha1.ClusterFolder, error) {

	for _, child := range curFolder.Spec.ChildClusterFolders {
		childClusterFolder := &v1alpha1.ClusterFolder{}
		name := client.ObjectKey{Name: child}

		err := r.Client.Get(ctx, name, childClusterFolder)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, err
			}
			continue
		}

		if childClusterFolder.Name == loopedFolder.Name {
			return childClusterFolder, nil
		}

		curWinner, err := r.loopChainWinner(ctx, loopedFolder, childClusterFolder)
		if err != nil {
			return nil, err
		} else if curWinner == nil {
			return nil, nil
		}

		curTS, ok := curWinner.Labels[ClusterFolderOwnershipClaimTimestampLabel]
		if !ok {
			return curWinner, nil
		}

		childTS, ok := childClusterFolder.Labels[ClusterFolderOwnershipClaimTimestampLabel]
		if !ok {
			return childClusterFolder, nil
		}

		curTSInt, err := strconv.ParseInt(curTS, 10, 64)
		if err != nil {
			return curWinner, nil
		}

		childTSInt, err := strconv.ParseInt(childTS, 10, 64)
		if err != nil {
			return childClusterFolder, nil
		}

		if curTSInt > childTSInt {
			return curWinner, nil
		}

		return childClusterFolder, nil
	}

	return nil, nil
}

// Lazy rectify of loops by removing the most recent claim in the chain.
//
// The controller handles loops by removing the item in the looped chain that
// is most recent. This has the effect of breaking the loop and orphaning
// the folder that caused the loop.
//
// While this will make the folder consistent, it can lead to
// unexpected outcomes for users who didn't realize they were creating
// a looped folder hierarchy. Ideally we want a validating webhook to
// catch these loops before they occur, but given that these are distributed
// objects being worked on across multiple threads (both the controller threads
// and validating webhook threads) we can't guarantee a loop won't occur.
//
// Strictly guaranteeing no loops are introduced would require distributed locking
// and expensive non-cached lookups.
func (r *ClusterFolderReconciler) reconcileClusterFolderLoop(ctx context.Context, loopedFolder *v1alpha1.ClusterFolder) error {

	winner, err := r.loopChainWinner(ctx, loopedFolder, loopedFolder)
	if err != nil {
		return err
	} else if winner == nil {
		return nil
	}
	ownerName, exists := winner.Labels[ClusterFolderOwnershipNameLabel]
	if !exists {
		return nil
	}

	return r.clearOwnership(ctx, ownerName, "", winner.Name)
}

func (r *ClusterFolderReconciler) getAllNamespaces(ctx context.Context, folder *v1alpha1.ClusterFolder, observed map[string]struct{}) ([]corev1.Namespace, *v1alpha1.ClusterFolder, error) {
	folderNamespaces := []corev1.Namespace{}
	for _, nsName := range folder.Spec.Namespaces {
		ns := corev1.Namespace{}
		name := client.ObjectKey{Name: nsName}
		err := r.Client.Get(ctx, name, &ns)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return folderNamespaces, nil, err
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
				return folderNamespaces, nil, err
			}
			continue
		}

		_, exists := observed[childClusterFolder.Name]
		if exists {
			return folderNamespaces, &childClusterFolder, nil
		}
		observed[childClusterFolder.Name] = struct{}{}

		childNamespaces, loopedChild, err := r.getAllNamespaces(ctx, &childClusterFolder, observed)
		if err != nil {
			return folderNamespaces, nil, err
		} else if loopedChild != nil {
			return folderNamespaces, loopedChild, nil
		}

		folderNamespaces = append(folderNamespaces, childNamespaces...)
	}

	return folderNamespaces, nil, nil
}

// +kubebuilder:rbac:groups=kubevirtfolderview.kubevirt.io.github.com,resources=folders,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubevirtfolderview.kubevirt.io.github.com,resources=folders/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubevirtfolderview.kubevirt.io.github.com,resources=folders/finalizers,verbs=update
func (r *ClusterFolderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logger.FromContext(ctx)

	folder := &v1alpha1.ClusterFolder{}

	log.Info(fmt.Sprintf("Reconciling cluster folder [%s]", req.NamespacedName.Name))
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
	folderNamespaces, loopedChildFolderDetected, err := r.getAllNamespaces(ctx, folder, map[string]struct{}{})
	if err != nil {
		return ctrl.Result{}, err
	} else if loopedChildFolderDetected != nil {
		err := r.reconcileClusterFolderLoop(ctx, loopedChildFolderDetected)
		if err != nil {
			return ctrl.Result{}, err
		}
		log.Info(fmt.Sprintf("Loop detected in cluster folder [%s], requeuing", loopedChildFolderDetected.Name))
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
