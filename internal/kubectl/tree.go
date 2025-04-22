package kubectl

import (
	"context"
	"fmt"
	"os"
	"strings"

	v1alpha1 "github.com/davidvossel/kubevirt-folder-view/api/v1alpha1"
	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	//corev1 "k8s.io/api/core/v1"
)

func printTree(root *v1alpha1.FolderIndex,
	childParent map[string]string,
	parentClusterFolder string,
	parentNamespace string,
	parentNamespacedFolder string,
	indention string) {

	if parentClusterFolder != "" {
		fmt.Printf("%s* ClusterFolder: [%s]\n", indention, parentClusterFolder)
		node, ok := root.Spec.ClusterFolderEntries[parentClusterFolder]
		if !ok {
			return
		}

		indention = fmt.Sprintf("%s  ", indention)

		for _, ns := range node.Namespaces {
			printTree(root, childParent, "", ns, "", indention)
		}
		for _, child := range node.ChildFolders {
			printTree(root, childParent, child, "", "", indention)
		}
	} else if parentNamespace != "" {
		fmt.Printf("%s* Namespace: [%s]\n", indention, parentNamespace)
		namespaceParentKey := fmt.Sprintf("NAMESPACE:%s", parentNamespace)
		indention = fmt.Sprintf("%s  ", indention)
		for childFolder, parentNS := range childParent {
			if parentNS != namespaceParentKey {
				continue
			}
			printTree(root, childParent, "", "", childFolder, indention)
		}
	} else if parentNamespacedFolder != "" {
		node, ok := root.Spec.NamespacedFolderEntries[parentNamespacedFolder]
		if !ok {
			return
		}

		fmt.Printf("%s* NamespacedFolder: [%s]\n", indention, parentNamespacedFolder)

		indention = fmt.Sprintf("%s  ", indention)

		for _, vm := range node.VirtualMachines {
			fmt.Printf("%s* VM: [%s]\n", indention+"  ", vm)
		}
		for _, child := range node.ChildFolders {
			printTree(root, childParent, "", "", child, indention)
		}
	}
}

var treeCmd = &cobra.Command{
	Use:   "tree TREE",
	Short: "Display folder tree view",
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		cl, err := client.New(config.GetConfigOrDie(), client.Options{})
		if err != nil {
			fmt.Println("failed to create client")
			os.Exit(1)
		}

		root := &v1alpha1.FolderIndex{}
		err = cl.Get(ctx, client.ObjectKey{Name: "root"}, root)
		if err != nil {
			fmt.Printf("failed to find root folder index: %v\n", err)
			os.Exit(1)
		}

		childParent := map[string]string{}
		rootClusterFolders := map[string]struct{}{}
		rootNamespacedFolders := map[string]struct{}{}

		// Discover the root folders
		// start by discovering all child parent relationships
		for parent, clusterEntry := range root.Spec.ClusterFolderEntries {
			for _, child := range clusterEntry.ChildFolders {
				childParent[child] = parent
				delete(rootClusterFolders, child)
			}

			_, isChild := childParent[parent]
			if !isChild {
				rootClusterFolders[parent] = struct{}{}
			}
		}

		for parent, namespacedEntry := range root.Spec.NamespacedFolderEntries {
			namespace := strings.Split(parent, "/")[0]
			namespaceParentKey := fmt.Sprintf("NAMESPACE:%s", namespace)

			for _, child := range namespacedEntry.ChildFolders {
				childParent[child] = parent
				delete(rootNamespacedFolders, child)
			}

			_, isChild := childParent[parent]
			if !isChild {
				rootNamespacedFolders[parent] = struct{}{}
				childParent[parent] = namespaceParentKey
			}
		}

		for parent, _ := range rootClusterFolders {
			printTree(root, childParent, parent, "", "", "")
		}

		// TODO print Namespaces and VMs within namespaces that are not in folders
	},
}

func newTreeCmd() *cobra.Command {
	return treeCmd
}
