# Overview

This project introduces the concept of a `folder` within Kubernetes to organize KubeVirt VirtualMachines and simplify the expression of RBAC for VirtualMachine access.

# Key Concepts

## FolderIndex

The **FolderIndex** object contains all the parent and child relationships between ClusterFolders and NamespacedFolders. This centralized object can be thought of the database from which the folder tree is organized. When a change is made to any folder regarding the folder's relationship to owning objects, the FolderIndex is the entity that is modified to apply this change.

## ClusterFolders

A **ClusterFolder** works at the cluster scope may contain both Namespaces and other nested ClusterFolders. Permissions added to a ClusterFolder are applied to all the Namespaces contained within the ClusterFolder and its nested child ClusterFolders.

## NamespacedFolders

A **NamespacedFolder** works at the namespace scope may contain VirtualMachines and other nested NamespacedFolders within that Namespace. Permissions added to a NamespacedFolder are applied to all the VirtualMachines contained within the NamespacedFolder and its nested child NamespacedFolders.

# Example: Folder Hierarchy in Practice. Modeling Development and Operation Teams

Folders can help organizations map their internal teams structure to their cluster infrastructure.

For example, let's say we have an organization with two departments, Development and Operations. The VirtualMachines and access control for VirtualMachines can be expressed using folders.

## Example Continued... Operation Team

We could start this example by modeling the Operations team. In this case, Operations has two environments, Staging and Production. This could be modeled using ClusterFolders and Namespaced to manage access to VMs across multiple namespaces. The tree view of this hierarchy would visually look like the figure below.

```
- ClusterFolder: infra-admins
    - ClusterFolder: operations
        - ClusterFolder: production
            - Namespace: prod-web-apps
                - NamespacedFolder: prod-web-app-a
                    - VM: web-app-a
                    - VM: web-app-a-db
                - NamespacedFolder: prod-web-app-b
                    - VM: web-app-b
                    - VM: web-app-b-db
        - ClusterFolder: staging
            - Namespace: staging-web-apps
                - NamespacedFolder: staging-web-app-a
                    - VM: web-app-a
                    - VM: web-app-a-db
                - NamespacedFolder: staging-web-app-b
                    - VM: web-app-b
                    - VM: web-app-b-db
```

In yaml form, the root FolderIndex object that represents this folder hierarchy in the backend API would look like the figure below.

```yaml
apiVersion: kubevirtfolderview.kubevirt.io.github.com/v1alpha1
kind: FolderIndex
metadata:
  name: root
spec:
  clusterFolderEntries:
    infra-admins:
      childFolders:
        - operations
    operations:
      childFolders:
        - production
        - staging
    production:
      namespaces:
        - prod-web-apps
    staging:
      namespaces:
        - staging-web-apps
  namespacedFolderEntries:
    prod-web-apps/prod-web-app-a:
      virtualMachines:
        - web-app-a
        - web-app-a-db
    prod-web-apps/prod-web-app-b:
      virtualMachines:
        - web-app-b
        - web-app-b-db
    staging-web-apps/staging-web-app-a:
      virtualMachines:
        - web-app-a
        - web-app-a-db
    staging-web-apps/staging-web-app-b:
      virtualMachines:
        - web-app-b
        - web-app-b-db
```

We want the Operations team members to have full access to the entire operations environment. This is achieved by mapping the `admin` role to a group called `operation-team` and applying that to the ClusterFolder called `operations`. The operations ClusterFolder would look like the following.

```yaml
apiVersion: kubevirtfolderview.kubevirt.io.github.com/v1alpha1
kind: ClusterFolder
metadata:
  name: operations
spec:
  folderPermissions:
  - subject:
      kind: Group
      name: operation-team
    roleRefs:
    - apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: admin
```

## Example continued... Development Teams

Now let's say there are two development teams. One team is responsible for web-app-a and the other is responsible for web-app-b. These teams need the access to debug their application in production, but we only want to grant each team access to the VMs they are responsible for.

This can be achieved using NamespacedFolders within the `prod-web-apps` namespace to give each team access to only the specific VirtualMachines hosting their application.

```yaml
apiVersion: kubevirtfolderview.kubevirt.io.github.com/v1alpha1
kind: NamespacedFolder
metadata:
  name: prod-web-app-a
  namespace: prod-web-apps
spec:
  folderPermissions:
  - subject:
      kind: Group
      name: dev-team-a
    roleRefs:
    - apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: edit
---
apiVersion: kubevirtfolderview.kubevirt.io.github.com/v1alpha1
kind: NamespacedFolder
metadata:
  name: prod-web-app-b
  namespace: prod-web-apps
spec:
  folderPermissions:
  - subject:
      kind: Group
      name: dev-team-b
    roleRefs:
    - apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: edit
```

## Example continued... Moving VMs between folders

Using NamespacedFolders, it is possible to dynamically move VMs between NamespacedFolders that have the same parent namespace. This can be useful for temporarily granting or revoking user access to a VirtualMachine.

Returning to our example, let's say the operations team needs to temporarily isolate a VirtualMachine in production so that no dev team can access the VirtualMachine. This can be achieved by creating a new NamespacedFolder within the `prod-web-apps` namespace and moving that VirtualMachine into the folder. As an example, let's say the `web-app-b` VirtualMachine needs to be isolated. The yaml for the temporary NamespacedFolder and resulting changes to the root FolderIndex would look like the figure below.


```yaml
apiVersion: kubevirtfolderview.kubevirt.io.github.com/v1alpha1
kind: NamespacedFolder
metadata:
  name: temp-folder-debug
  namespace: prod-web-apps
spec:
---
apiVersion: kubevirtfolderview.kubevirt.io.github.com/v1alpha1
kind: FolderIndex
metadata:
  name: root
spec:
  clusterFolderEntries:
    infra-admins:
      childFolders:
        - operations
    operations:
      childFolders:
        - production
        - staging
    production:
      namespaces:
        - prod-web-apps
    staging:
      namespaces:
        - staging-web-apps
  namespacedFolderEntries:
    prod-web-apps/temp-folder-debug:
      virtualMachines:
        - web-app-b
    prod-web-apps/prod-web-app-a:
      virtualMachines:
        - web-app-a
        - web-app-a-db
    prod-web-apps/prod-web-app-b:
      virtualMachines:
        - web-app-b-db
    staging-web-apps/staging-web-app-a:
      virtualMachines:
        - web-app-a
        - web-app-a-db
    staging-web-apps/staging-web-app-b:
      virtualMachines:
        - web-app-b
        - web-app-b-db
```

Since the operations team already has broad permissions to access all VirtualMachines within the `operations` ClusterFolder, there's no need explicitly grant the operation team access to the `temp-folder-debug` folder as that permission is already inherited through the folder hierarchy.

If the operations team wanted to grant a single member of the development team access to this temporary folder, that could be accomplished by adding the folder permission to the NamespacedFolder. The resulting yaml would look like this.

```yaml
apiVersion: kubevirtfolderview.kubevirt.io.github.com/v1alpha1
kind: NamespacedFolder
metadata:
  name: temp-folder-debug
  namespace: prod-web-apps
spec:
  folderPermissions:
  - subject:
      kind: user
      name: steve
    roleRefs:
    - apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: edit
```
