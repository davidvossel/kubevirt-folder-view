# kubevirt-folder-view


This project creates the concept of a `folder` within kubernetes to organize namespaces.

ClusterFolders contain Namespaces and nested child folders. Permissions can be assigned to a folder, which inturn results in those permissions being applied to all namespaces within the folder including the namespaces within the child folders.

## Example

In this example, a root folder called `admin-folder` is created and multiple nested child folders are created within that root folder. The `admin-folder` results in the group `admins` having admin privileges to all the namespaces in the root and children folders. The nested folders have more fine granular permissions scoped to a smaller set of namespaces and groups.

The admin-folder is a root folder and gives admin access to all the namespaces in both the admin-folder and child folders.

```yaml
apiVersion: kubevirtfolderview.kubevirt.io.github.com/v1alpha1
kind: ClusterFolder
metadata:
  name: admin-folder
spec:
  namespaces:
  - kube-system
  childClusterFolders:
  - ops-support
  - devs
  folderPermissions:
  - subject:
      kind: Group
      name: admins
    roleRefs:
    - apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: admin
```

The `ops-support` folder is a child of the admin-folder, and gives view access to level1 support and edit access to level2 support

```yaml
apiVersion: kubevirtfolderview.kubevirt.io.github.com/v1alpha1
kind: ClusterFolder
metadata:
  name: ops-support
spec:
  namespaces:
  - prod1
  - prod2
  folderPermissions:
  - subject:
      kind: Group
      name: level1-support
    roleRefs:
    - apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: view
  - subject:
      kind: Group
      name: level2-support
    roleRefs:
    - apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: edit
```

The `devs` folder is a child of admin-folder, and gives admin access to a subset of dev namespaces

```yaml
apiVersion: kubevirtfolderview.kubevirt.io.github.com/v1alpha1
kind: ClusterFolder
metadata:
  name: devs
spec:
  namespaces:
  - dev1
  - dev2
  folderPermissions:
  - subject:
      kind: Group
      name: developers
    roleRefs:
    - apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: admin
```

