# Demo with Longhorn

We will demonstrate how Union CSI can be integrated with the [Longhorn](https://longhorn.io/)
block storage system to obtain a Union CSI volume composed from Longhorn volumes
allocated on different nodes in the cluster. Longhorn is a fitting match for
Union CSI as its storage provider due to its ability to remotely attach and
access its volumes from any node over Ethernet using iSCSI.

## Table Of Contents

* [Environment](#environment)
* [Longhorn StorageClass](#longhorn-storageclass)
* [Creating a Large Longhorn Volume](#creating-a-large-longhorn-volume)
* [Union CSI StorageClass](#union-csi-storageclass)
* [Creating a Union CSI Volume](#creating-a-union-csi-volume)
* [Attaching and Mounting a Union CSI Volume](#attaching-and-mounting-a-union-csi-volume)
* [Utilizing a Union CSI Volume](#utilizing-a-union-csi-volume)
* [Summary](#summary)


## Environment

Kubernetes cluster with 2 nodes, each equipped with a 128 GiB local SSD.

## Longhorn StorageClass

The Longhorn installation includes a default StorageClass for creating Longhorn
volumes using PVCs. First, we set the number of replicas Longhorn creates for
each volume to 1, through the `numberOfReplicas` parameter of the StorageClass.
This ensures that only one (primary) replica is created for every Longhorn
volume thus saving space. The Longhorn StorageClass in YAML looks like this:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: longhorn-1-replica
parameters:
  dataLocality: disabled
  fromBackup: ""
  fsType: ext4
  numberOfReplicas: "1"
  staleReplicaTimeout: "30"
provisioner: driver.longhorn.io
reclaimPolicy: Delete
allowVolumeExpansion: true
volumeBindingMode: Immediate
```

## Creating a Large Longhorn Volume

Our cluster is equipped with two 128 GiB SSDs, totalling at 256 GiB. Assume our
application requires a 200 GiB volume. Let's find out how Longhorn will respond.
Below is a PVC that points to the Longhorn StorageClass and requests a 200 GiB
volume from Longhorn.

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: longhorn-pvc
spec:
  accessModes:
  - ReadWriteOnce
  storageClassName: longhorn-1-replica
  resources:
    requests:
      storage: 200Gi
```

1. Create the Longhorn PVC:

```sh
kubectl apply -f longhorn-pvc.yaml
```

2. Check the PVC and corresponding PV:

```console
$ kubectl get pvc longhorn-pvc 
NAME          STATUS  VOLUME                                    CAPACITY  ACCESS MODES  STORAGECLASS        AGE
longhorn-pvc  Bound   pvc-76b412fc-e2af-475f-85d6-3f9091cf5b74  200Gi     RWO           longhorn-1-replica  3s

$ kubectl get pv
NAME                                      CAPACITY  ACCESS MODES  RECLAIM POLICY  STATUS  CLAIM                 STORAGECLASS        REASON  AGE
pvc-76b412fc-e2af-475f-85d6-3f9091cf5b74  200Gi     RWO           Delete          Bound   default/longhorn-pvc  longhorn-1-replica          4s
```

We discover that the Longhorn PVC is bound to a PV. At first glance, it appears
that we have successfully created a 200 GiB volume. However, let's take a closer
look.

3. Check the Longhorn Replica custom resource corresponding to the volume:

```console
$ kubectl get -n longhorn-system replica
NAME                                                 STATE    NODE  DISK  INSTANCEMANAGER  IMAGE  AGE
pvc-76b412fc-e2af-475f-85d6-3f9091cf5b74-r-18051260  stopped                                      9m4s
```

We notice that the `NODE` and `DISK` columns are missing a value. This indicates
that Longhorn failed to schedule the replica on a node and allocate the required
block device.

4. Inspect the logs of the Longhorn Manager Pod which handled the volume
provisioning request delegated by the Longhorn CSI driver:

```console
$ kubectl logs -n longhorn-system longhorn-manager-9d5m5
...
time="2023-11-26T17:47:27Z" level=error msg="There's no available disk for replica pvc-76b412fc-e2af-475f-85d6-3f9091cf5b74-r-18051260, size 214748364800"
time="2023-11-26T17:47:27Z" level=warning msg="Failed to schedule replica" accessMode=rwo controller=longhorn-volume frontend=blockdev migratable=false node=aks-agentpool-30515490-vmss00000q owner=aks-agentpool-30515490-vmss00000q replica=pvc-76b412fc-e2af-475f-85d6-3f9091cf5b74-r-18051260 state=detached volume=pvc-76b412fc-e2af-475f-85d6-3f9091cf5b74
...
```

The logs reveal repeated reports stating insufficient available disk space for
the requested replica. As expected, a 200 GiB Longhorn volume is not feasible on
128 GiB disks. If we were to create a Pod using this PVC, the Pod will fail to
get scheduled on a node because the desired capacity cannot be fullfiled.

In the following sections, we showcase how Union CSI can help overcome this
capacity limitation, enabling the utilization of a 200 GiB volume composed of
two smaller 100 GiB Longhorn volumes on separate nodes.

## Union CSI StorageClass

To configure Union CSI to use Longhorn as its lower plugin, we first need to
configure and create the appropriate StorageClass. The StorageClass listed below
specifies the name of the Union CSI plugin, `union.csi.driver.union.io`, in the
`provisioner` field and the name of the Longhorn StorageClass we created
earlier, `longhorn‐1‐replica`, through the parameter `lowerStorageClassName`.
This way, Union CSI can create PVCs using the Longhorn StorageClass and have
Longhorn create the braches for the Union CSI volume.

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: union-storage-longhorn
provisioner: union.csi.driver.union.io
parameters:
  lowerStorageClassName: longhorn-1-replica
reclaimPolicy: Delete
volumeBindingMode: Immediate
```

## Creating a Union CSI Volume

After cleaning up the PVC created in [Creating a Large Longhorn Volume](#creating-a-large-longhorn-volume),
we proceed to create a new PVC requesting a 200 GiB volume, this time using the
Union CSI StorageClass.

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: union-pvc-longhorn
spec:
  storageClassName: union-storage-longhorn
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 200Gi
```

1. Create the Union CSI PVC:

```sh
kubectl apply -f union-pvc-longhorn.yaml
```

2. Check the PVC:

```console
$ kubectl get pvc union-pvc-longhorn 
NAME                STATUS  VOLUME                                    CAPACITY  ACCESS MODES  STORAGECLASS            AGE
union-pvc-longhorn  Bound   pvc-ff5fca91-682b-48e9-959a-4a38ee5e2bf0  200Gi     RWO           union-storage-longhorn  12s
```

The upper PVC is bound to a PV.

3. Check the PVCs created by Union CSI in the `union` namespace:

```console
$ kubectl get -n union pvc
NAME                                                  STATUS  VOLUME                                    CAPACITY  ACCESS MODES  STORAGECLASS        AGE
split-pvc-ff5fca91-682b-48e9-959a-4a38ee5e2bf0-k2p7n  Bound   pvc-75a2e35b-0488-41d0-ab2b-f85d0b35632c  100Gi      RWO           longhorn-1-replica  35s
split-pvc-ff5fca91-682b-48e9-959a-4a38ee5e2bf0-mst9w  Bound   pvc-bb282fc0-7d58-4ce1-872c-015eb05967c4  100Gi      RWO           longhorn-1-replica  35s
```

We can see that Union CSI has created 2 lower, equally sized PVCs of 100 GiB (as
described in [Demo Version](https://github.com/on2e/union-csi/tree/demo?tab=readme-ov-file#demo-version)
using the Longhorn StorageClass, which are bound to their respective PVs.

4. Check the PVs:

```console
$ kubectl get pv
NAME                                      CAPACITY  ACCESS MODES  RECLAIM POLICY  STATUS  CLAIM                                                       STORAGECLASS            REASON  AGE
pvc-75a2e35b-0488-41d0-ab2b-f85d0b35632c  100Gi     RWO           Delete          Bound   union/split-pvc-ff5fca91-682b-48e9-959a-4a38ee5e2bf0-k2p7n  longhorn-1-replica              13s
pvc-bb282fc0-7d58-4ce1-872c-015eb05967c4  100Gi     RWO           Delete          Bound   union/split-pvc-ff5fca91-682b-48e9-959a-4a38ee5e2bf0-mst9w  longhorn-1-replica              13s
pvc-ff5fca91-682b-48e9-959a-4a38ee5e2bf0  200Gi     RWO           Delete          Bound   default/union-pvc-longhorn                                  union-storage-longhorn          16s
```

Notice the 200 GiB Union CSI PV and its two 100 GiB Longhorn branches.

5. Check the Longhorn Replicas corresponding to the two Longhorn volumes:

```console
$ kubectl get -n longhorn-system replica
NAME                                                 STATE    NODE                               DISK                                  INSTANCEMANAGER  IMAGE  AGE
pvc-75a2e35b-0488-41d0-ab2b-f85d0b35632c-r-34eeca8c  stopped  aks-agentpool-30515490-vmss00000q  61233fcc-1261-41ff-9f0c-410c166ce168                          26m
pvc-bb282fc0-7d58-4ce1-872c-015eb05967c4-r-6208e7af  stopped  aks-agentpool-30515490-vmss00000p  d551e4b0-5ec9-422a-a5e3-04e66c948af4                          26m
```

The output above indicates that the two replicas have been successfully
scheduled on different nodes and have been assigned a block device by Longhorn
(check the `NODE` and `DISK` columns).

Although the upper 200 GiB PV exists in Kubernetes, the underlying Union CSI
volume is not yet materialized. To merge the two branches together and utilize
the resulting volume on a node, we first need to create a Pod to consume the
volume.

## Attaching and Mounting a Union CSI Volume

Listed below is a Pod that uses the Union CSI volume through its PVC and
specifies the `/data` directory as the mount point for the volume inside an
NGINX container.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: union-pod-longhorn
spec:
  containers:
  - name: app
    image: nginx:stable-alpine
    imagePullPolicy: IfNotPresent
    volumeMounts:
    - name: vol
      mountPath: /data
    ports:
    - containerPort: 80
  volumes:
  - name: vol
    persistentVolumeClaim:
      claimName: union-pvc-longhorn
```

1. Create the Pod:

```sh
kubectl apply -f union-pod-longhorn
```

2. Check the Pod:

```console
$ kubectl get pod union-pod-longhorn
NAME                READY  STATUS   RESTARTS  AGE
union-pod-longhorn  1/1    Running  0         35s
```

The NGINX Pod is running.

3. Check the Pod created by Union CSI in the `union` namespace to merge and
mount the Union CSI volume:

```console
$ kubectl get -n union pod pod-pvc-ff5fca91-682b-48e9-959a-4a38ee5e2bf0
NAME                                          READY  STATUS   RESTARTS  AGE
pod-pvc-ff5fca91-682b-48e9-959a-4a38ee5e2bf0  1/1    Running  0         43s
```

When the consumer Pod utilizing the Union CSI volume gets scheduled on a node,
Union CSI creates an internal Pod as shown above. This Pod uses the two Longhorn
branches inside a container with the `mergerfs` program and assigns the Pod to
the same node. Since our cluster consists of only 2 nodes, and each node has a
local Longhorn volume, when the internal Pod is scheduled on the specified node,
Longhorn attaches its two volumes there, one locally and the other remotely from
the opposite node through the network. Once the branches are ready and the
internal Pod starts, `mergerfs` is executed inside the container to merge the
branches and mount the Union CSI volume on the host, making it available for
workloads.

## Utilizing a Union CSI Volume

1. Connect to the NGINX container of the consumer Pod and inspect its contents:

```console
$ kubectl exec -it union-pod-longhorn -- /bin/sh

# ls -ild data
12101348768806803897 drwxr-xr-x    3 root     root          4096 Nov 28 17:33 data

# ls -ila /data
total 24
12101348768806803897 drwxr-xr-x    3 root     root          4096 Nov 28 17:33 .
             3873171 drwxr-xr-x    1 root     root          4096 Nov 28 17:33 ..
 4938303994640588972 drwx------    2 root     root         16384 Nov 28 17:33 lost+found
```

The `mergerfs` filesystem is mounted inside the NGINX container at `/data`.
Notice the unusually large inode numbers for `/data` and its contents.

2. Use `dd` to write two 10 GiB files (`10GB.file` and `other10GB.file`) under
`/data`:

```console
# dd if=/dev/zero of=/data/10GB.file bs=1M count=10240 status=progress
...
# dd if=/dev/zero of=/data/other10GB.file bs=1M count=10240 status=progress
...
# ls -ila /data
total 20971552
12101348768806803897 drwxr-xr-x 3 root root        4096 Nov 28 18:06 .
             3873171 drwxr-xr-x 1 root root        4096 Nov 28 17:33 ..
 9581384014260166741 -rw-r--r-- 1 root root 10737418240 Nov 28 18:06 10GB.file
 4938303994640588972 drwx------ 2 root root       16384 Nov 28 17:33 lost+found
17568198116042200688 -rw-r--r-- 1 root root 10737418240 Nov 28 18:08 other10GB.file
```

3. Connect to the `mergerfs` container of the internal Pod and inspect its
contents:

```console
$ kubectl exec -it -n union pod-pvc-ff5fca91-682b-48e9-959a-4a38ee5e2bf0 -- /bin/sh

# ls -ila /volume
total 16
             1846804 drwxr-xr-x    4 root     root          4096 Nov 28 17:33 .
             1846803 drwxr-xr-x    3 root     root          4096 Nov 28 17:33 ..
             1846806 drwxr-xr-x    4 root     root          4096 Nov 28 17:33 branches
12101348768806803897 drwxr-xr-x    3 root     root          4096 Nov 28 18:06 merged

# ls -ila /volume/merged
total 20971552
12101348768806803897 drwxr-xr-x    3 root     root            4096 Nov 28 18:06 .
             1846804 drwxr-xr-x    4 root     root            4096 Nov 28 17:33 ..
 9581384014260166741 -rw-r--r--    1 root     root     10737418240 Nov 28 18:06 10GB.file
 4938303994640588972 drwx------    2 root     root           16384 Nov 28 17:33 lost+found
17568198116042200688 -rw-r--r--    1 root     root     10737418240 Nov 28 18:08 other10GB.file

# ls -ila /volume/branches/
total 16
1846806 drwxr-xr-x    4 root     root          4096 Nov 28 17:33 .
1846804 drwxr-xr-x    4 root     root          4096 Nov 28 17:33 ..
      2 drwxr-xr-x    3 root     root          4096 Nov 28 18:06 branch0-split-pvc-ff5fca91-682b-48e9-959a-4a38ee5e2bf0-k2p7n
      2 drwxr-xr-x    3 root     root          4096 Nov 28 18:05 branch1-split-pvc-ff5fca91-682b-48e9-959a-4a38ee5e2bf0-mst9w
```

The `mergerfs` filesystem is mounted within the container under
`/volume/merged`. Each individual branch is mounted under `/volume/branches`.

> *Note*: The filesystem is propagated from within the container to the host
> using [bidirectional mount propagation](https://kubernetes.io/docs/concepts/storage/volumes/#mount-propagation)
> on the specification of the internal Pod.

4. Inspect the branches:

```console
# ls -ila /volume/branches/branch0-split-pvc-ff5fca91-682b-48e9-959a-4a38ee5e2bf0-k2p7n/
total 10485788
      2 drwxr-xr-x    3 root     root            4096 Nov 28 18:06 .
1846806 drwxr-xr-x    4 root     root            4096 Nov 28 17:33 ..
     11 drwx------    2 root     root           16384 Nov 28 17:33 lost+found
     12 -rw-r--r--    1 root     root     10737418240 Nov 28 18:08 other10GB.file

# ls -ila /volume/branches/branch1-split-pvc-ff5fca91-682b-48e9-959a-4a38ee5e2bf0-mst9w/
total 10485788
      2 drwxr-xr-x    3 root     root            4096 Nov 28 18:05 .
1846806 drwxr-xr-x    4 root     root            4096 Nov 28 17:33 ..
     12 -rw-r--r--    1 root     root     10737418240 Nov 28 18:06 10GB.file
     11 drwx------    2 root     root           16384 Nov 28 17:33 lost+found
```

We can observe that the two 10 GiB files we created earlier inside the
`mergerfs` filesystem from within the NGINX container have been routed by
`mergerfs` (the FUSE daemon) to separate underlying branches. This demonstrates
that by using the Union CSI volume we have effectively drawn storage from
different disks and nodes.

## Summary

By integrating Union CSI with Longhorn, we have successfully created, attached,
mounted, and utilized a 200 GiB volume composed of two 100 GiB Longhorn volumes
allocated on different nodes.

Initially, creating a 200 GiB Longhorn block device volume was not feasible on a
cluster with 128GiB local SSDs, as it exceeded the available disk space. By
incorporating the Union CSI volume plugin, the 200 GiB volume request was split
in half through storage claims (PVCs) made on the Longhorn system, allowing
Longhorn to provision the two smaller 100 GiB volumes successfully. Union CSI
then merged the two lower halves using the `mergerfs` tool and mounted the union
filesystem on the specified node. Any underlying filesystems on nodes remote to
the target node were accessed by Longhorn through its iSCSI feature.
