This is an initial, rough proof-of-concept version aimed at quickly obtaining
a demo while still in the process of learning Go and Kubernetes. It is mostly
undocumented and untested. It will remain here in a frozen state. A new
version will commence development in branch [`main`](https://github.com/on2e/union-csi/tree/main).

# Union CSI

Union CSI is a [Container Storage Interface](https://github.com/container-storage-interface/spec/blob/master/spec.md)
(CSI) plugin for Kubernetes, enabling the combination of multiple persistent
volumes (branches) into single, unified hierarchies as [FUSE](https://www.kernel.org/doc/html/latest/filesystems/fuse.html)
union mounts for Pods.

## Table Of Contents

* [Goals](#goals)
* [Design](#design)
    * [Multi-node Volumes](#multi-node-volumes)
    * [Demo Version](#demo-version)
* [Terminology](#terminology)
* [Performance](#performance)
* [Building](#building)
* [Deployment](#deployment)
* [Documentation](#documentation)
* [WIP](#wip)

## Goals

1. Merge existing Kubernetes persistent volumes to create a new union
filesystem volume.
2. Create new volumes to merge based on user configuration.
3. Support generic storage composition (e.g., cached or tiered storage,
`bcachefs`).

## Design

Union CSI is a CSI "meta-plugin": a CSI plugin that aggregates the volumes of
other plugins. This means than an underlying or *lower* storage driver is
needed to perform the actual work (i.e., provision, attach, mount, etc.). Users
point Union CSI towards a lower plugin to request and combine storage through
its [StorageClass](https://kubernetes.io/docs/concepts/storage/storage-classes/)
object. The lower plugin does not have to be CSI compliant.

Union CSI invokes the creation and deletion of volumes from the lower plugin
through the [PersistentVolumeClaim](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#persistentvolumeclaims)
(PVC) API. Users can create a PVC (upper) that points to the Union CSI plugin
and specify the desired number and size of branches from the lower plugin, or
indicate existing branches to collect. Union CSI will then create or adopt the
required set of PVCs (lower) that point to the lower plugin. Finally, it
utilizes [`mergerfs`](https://github.com/trapexit/mergerfs) to pool the
branches together on the target node and provide a FUSE filesystem to
workloads.

### Multi-node Volumes

The primary motivation behind Union CSI is to achieve *multi-node volumes*:
volumes whose branches span across different disks and nodes in the cluster.
This is accomplished by pooling the remote branches through the network.
This enables users to achieve capacities that near the total available capacity
of the cluster with just a single volume, promising massive scale-out and
scale-up potential.

Union CSI currently delegates the core of this feature to the lower plugin
&mdash; it is the lower plugin installed that should be able to remorely access
its storage (e.g. iSCSI) in order for Union CSI to leverage and combine storage
assets from different nodes.

Additionally, the only way to allocate new branches on different nodes through
Union CSI is for the user to request a volume large enough so that its branches
can only be placed on different nodes by the lower plugin.

### Demo Version

The demo version of Union CSI, found in this branch, statically splits the
requested capacity in half by always creating two equally sized lower PVCs. An
example showcasing how this mini version can be used to yield a powerful use
case is provided in [Demo with Longhorn](https://github.com/on2e/union-csi/blob/demo/docs/longhorn-demo.md),
where [Longhorn](https://longhorn.io) is employed as the lower storage provider
for Union CSI.

## Terminology

* **Upper**: Adjective for entities, components and resources related to the
Union CSI volume plugin, e.g., upper plugin, upper volume, upper PVC, upper PV,
etc.
* **Lower**: Adjective for entities, components and resources related to the
underlying volume plugin used by Union CSI, e.g., lower plugin, lower volume,
lower PVC, lower PV, etc.

## Performance

Since Union CSI utilizes `mergerfs` and therefore FUSE, there is the added FUSE
overhead of transitioning back and forth from userspace to kernelspace to be
expected from Union CSI volumes. For more information, please refer to the
[documentation](https://github.com/trapexit/mergerfs?tab=readme-ov-file#performance)
of `mergerfs`.

## Building

To create a local Docker image run

```sh
make IMAGE=union-csi:demo docker-build
```

## Deployment

To deploy this demo version of Union CSI in a Kubernetes cluster via Kustomize
using the Docker images from my [personal registry](https://hub.docker.com/search?q=on2e)
on Docker Hub, run:

```sh
kubectl apply -k ./deploy/k8s
```

## Documentation

* [Demo with Longhorn](https://github.com/on2e/union-csi/blob/demo/docs/longhorn-demo.md)

## WIP

Union CSI is still work in progress and highly experimental. It is currently a
personal and academic project.   
