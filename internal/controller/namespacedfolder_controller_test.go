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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubevirtfolderviewkubevirtiov1alpha1 "github.com/davidvossel/kubevirt-folder-view/api/v1alpha1"
)

var _ = Describe("NamespacedFolder Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		rootNamespacedName := types.NamespacedName{
			Name: "root",
		}

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		namespacedFolder := &kubevirtfolderviewkubevirtiov1alpha1.NamespacedFolder{}
		root := &kubevirtfolderviewkubevirtiov1alpha1.FolderIndex{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind NamespacedFolder")
			err := k8sClient.Get(ctx, typeNamespacedName, namespacedFolder)
			if err != nil && errors.IsNotFound(err) {
				resource := &kubevirtfolderviewkubevirtiov1alpha1.NamespacedFolder{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					// TODO(user): Specify other spec details if needed.
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			err = k8sClient.Get(ctx, rootNamespacedName, root)
			if err != nil && errors.IsNotFound(err) {
				resource := &kubevirtfolderviewkubevirtiov1alpha1.FolderIndex{
					ObjectMeta: metav1.ObjectMeta{
						Name: "root",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			err := k8sClient.Get(ctx, typeNamespacedName, namespacedFolder)
			Expect(err).NotTo(HaveOccurred())
			err = k8sClient.Get(ctx, rootNamespacedName, root)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance NamespacedFolder and root")
			Expect(k8sClient.Delete(ctx, namespacedFolder)).To(Succeed())
			Expect(k8sClient.Delete(ctx, root)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &NamespacedFolderReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
