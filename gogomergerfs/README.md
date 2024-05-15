# Gogomergerfs

A Go wrapper program for the `mergerfs` command. It is designed to be employed
by the Union CSI plugin to combine volumes into a `mergerfs` filesystem from
within a Pod and offer specific *quality-of-life* functionalities after the
filesystem is mounted.

## Usage

```console
$ gogomergerfs mergerfs --help
A featureful FUSE based union filesystem (https://github.com/trapexit/mergerfs)

Usage:
  gogomergerfs mergerfs [flags]

Flags:
      --branches strings   Comma-separated list of paths to merge together
      --target string      The union mount point
  -o, --options strings    Comma-separated list of mount options to pass directly to mergerfs
      --block              Execute mergerfs, block for SIGINT | SIGTERM, then unmount. If set to false, execute mergerfs as if executing directly the command
  -h, --help               help for mergerfs
```

## Design

The `gogomergerfs mergerfs` command syntactically mirrors and invokes the
original `mergerfs`, with one addition. If the `--block` argument is given,
`gogomergerfs` executes mergerfs and then blocks, waiting for the `SIGINT` or
`SIGTERM` signals. Upon receiving such a signal, `gogomergerfs` proceeds to
unmount the `mergerfs` filesystem from the target mount point before exiting.
For Union CSI, this:

* Allows the internal Pod that successfully runs `mergerfs` to remain running
(instead of terminating right away if were to run `mergerfs` directly), ensuring
that the FUSE filesystem remains active for as long as it is being used by a
consumer Pod.
* Enables the FUSE filesystem to be gracefully unmounted from the host when the
Pod is deleted by Union CSI and the container receives a `SIGTERM` signal from
Kubernetes to terminate.

## Building

To build into the same Docker image both the `gogomergerfs` and `mergerfs`
binaries, enabling the former to execute the latter when the container starts,
run:

```sh
make IMAGE=gogomergerfs:demo docker-build
```
