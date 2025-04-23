package kubectl

import (
	"context"
	"fmt"
	"os"
	"strings"

	v1alpha1 "github.com/davidvossel/kubevirt-folder-view/api/v1alpha1"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	virtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type printTreeData struct {
	root           *v1alpha1.FolderIndex
	childParentMap map[string]string
	vmToFolderMap  map[string]string

	vmNamespaceMap map[string][]string
	namespaceMap   map[string]struct{}
	vmMap          map[string]struct{}
}

func printTree(data *printTreeData,
	parentClusterFolder string,
	parentNamespace string,
	parentNamespacedFolder string,
	indention string) {

	if parentClusterFolder != "" {
		fmt.Printf("%s* ClusterFolder: [%s]\n", indention, parentClusterFolder)
		node, ok := data.root.Spec.ClusterFolderEntries[parentClusterFolder]
		if !ok {
			return
		}

		indention = fmt.Sprintf("%s  ", indention)

		for _, ns := range node.Namespaces {
			printTree(data, "", ns, "", indention)
		}
		for _, child := range node.ChildFolders {
			printTree(data, child, "", "", indention)
		}
	} else if parentNamespace != "" {
		_, exists := data.namespaceMap[parentNamespace]
		if !exists {
			return
		}
		fmt.Printf("%s* Namespace: [%s]\n", indention, parentNamespace)
		namespaceParentKey := fmt.Sprintf("NAMESPACE:%s", parentNamespace)
		for childFolder, parentNS := range data.childParentMap {
			if parentNS != namespaceParentKey {
				return
			}
			printTree(data, "", "", childFolder, indention+"  ")
		}

		vmList, exists := data.vmNamespaceMap[parentNamespace]
		if !exists {
			return
		}
		for _, vm := range vmList {
			_, isInFolder := data.vmToFolderMap[vm]
			if !isInFolder {
				fmt.Printf("%s* Namespace: [%s]\n", indention, parentNamespace)
			}

		}

	} else if parentNamespacedFolder != "" {
		node, ok := data.root.Spec.NamespacedFolderEntries[parentNamespacedFolder]
		if !ok {
			return
		}

		fmt.Printf("%s* NamespacedFolder: [%s]\n", indention, parentNamespacedFolder)

		for _, vm := range node.VirtualMachines {
			namespace := strings.Split(parentNamespacedFolder, "/")[0]
			_, exists := data.vmMap[fmt.Sprintf("%s/%s", namespace, vm)]
			if exists {
				fmt.Printf("%s* VM: [%s]\n", indention+"  ", vm)
			}
		}
		for _, child := range node.ChildFolders {
			printTree(data, "", "", child, indention+"  ")
		}
	}
}

var treeCmd = &cobra.Command{
	Use:   "tree TREE",
	Short: "Display folder tree view",
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		childParentMap := map[string]string{}
		rootClusterFolders := map[string]struct{}{}
		rootNamespacedFolders := map[string]struct{}{}

		vmToFolderMap := map[string]string{}
		namespaceToFolderMap := map[string]string{}

		// tells us if namespaces exist or not
		namespaceMap := map[string]struct{}{}

		// tells us if vms exist or not
		vmMap := map[string]struct{}{}

		// tells us if vms exist or not in a namespace
		vmNamespaceMap := map[string][]string{}

		cl, err := client.New(config.GetConfigOrDie(), client.Options{})
		if err != nil {
			fmt.Printf("failed to create client: %v\n", err)
			os.Exit(1)
		}

		var vmList virtv1.VirtualMachineList
		if err := cl.List(ctx, &vmList); err != nil {
			fmt.Printf("failed to list vms: %v\n", err)
			os.Exit(1)
		}
		for _, vm := range vmList.Items {
			vmMap[fmt.Sprintf("%s/%s", vm.Namespace, vm.Name)] = struct{}{}

			vmList, exists := vmNamespaceMap[vm.Namespace]
			if exists {
				vmNamespaceMap[vm.Namespace] = []string{vm.Name}
			} else {
				vmNamespaceMap[vm.Namespace] = append(vmList, vm.Name)
			}
		}

		var namespaceList corev1.NamespaceList
		if err := cl.List(ctx, &namespaceList); err != nil {
			fmt.Printf("failed to list namespaces: %v\n", err)
			os.Exit(1)
		}
		for _, ns := range namespaceList.Items {
			namespaceMap[ns.Name] = struct{}{}
		}

		root := &v1alpha1.FolderIndex{}
		err = cl.Get(ctx, client.ObjectKey{Name: "root"}, root)
		if err != nil {
			fmt.Printf("failed to find root folder index: %v\n", err)
			os.Exit(1)
		}

		// Discover the root folders
		// start by discovering all child parent relationships
		for parent, clusterEntry := range root.Spec.ClusterFolderEntries {
			for _, child := range clusterEntry.ChildFolders {
				childParentMap[child] = parent
				delete(rootClusterFolders, child)
			}

			for _, ns := range clusterEntry.Namespaces {
				namespaceToFolderMap[ns] = parent
			}
			_, isChild := childParentMap[parent]
			if !isChild {
				rootClusterFolders[parent] = struct{}{}
			}
		}

		for parent, namespacedEntry := range root.Spec.NamespacedFolderEntries {
			namespace := strings.Split(parent, "/")[0]
			namespaceParentKey := fmt.Sprintf("NAMESPACE:%s", namespace)

			for _, child := range namespacedEntry.ChildFolders {
				childParentMap[child] = parent
				delete(rootNamespacedFolders, child)
			}
			for _, vm := range namespacedEntry.VirtualMachines {
				vmToFolderMap[vm] = parent
			}

			_, isChild := childParentMap[parent]
			if !isChild {
				rootNamespacedFolders[parent] = struct{}{}
				childParentMap[parent] = namespaceParentKey
			}
		}

		for parent, _ := range rootClusterFolders {
			printTree(&printTreeData{
				root:           root,
				childParentMap: childParentMap,
				vmToFolderMap:  vmToFolderMap,
				vmNamespaceMap: vmNamespaceMap,
				vmMap:          vmMap,
				namespaceMap:   namespaceMap,
			}, parent, "", "", "")
			//printNonFolderedNamespaces( )
		}

		// TODO print Namespaces and VMs within namespaces that are not in folders
	},
}

func newTreeCmd() *cobra.Command {
	return treeCmd
}
