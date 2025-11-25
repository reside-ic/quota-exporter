# quota-exporter

A prometheus exporter that reports numbers from Linux disk quota.

The exporter needs to be pointed at a list of mountpoints for which quotas
should be reported. Both current usage and configured limits are reported in
the metrics.

Run the exporter with the following command:
```sh
quota-exporter --mountpoint /home
```

Multiple mountpoints can be included by repeating the `--mountpoint` option.
Alternatively, you can also run `quota-exporter` with the `--all` flag, which
will cause it to export quotas for all mountpoints that support it.

By default the exporter will listen on port 10018 and export the metrics
under the `/metrics` path. The port can be overriden using the `--listen` option.

The exporter needs to run as root (or with `CAP_SYS_ADMIN`). This is
required by the kernel's quota interface.

Currently only user quotas are supported.

## Testing

Configuring quotas requires modification to the host system to mount new disks
and enable quotas, as well as adding new users. It's not clear how reliable
this would be, even in the context of a Docker container.

Instead the tests are implemented as a QEMU VM, using the NixOS test framework.
See the [`test.nix`](./test.nix) file for details.

The tests can be run with the following command:

```sh
nix run .#vm-test
```

## Compatibility

### Kernel version

The implementation uses the `quotactl_fd` system call, which was introduced
in Linux 5.14. This is available in Ubuntu 22.04 and above, as well as
Debian bookworm and above.

It would be possible to support older kernel version by using the existing
`quotactl` system call, though the old interface is more tedious to use.

The old system call expects the path to the block device as an argument,
which requires the application to figure out which block device is behind
the given mount point. The new system call expects an `O_PATH` file
descriptor of the mountpoint instead.

### Filesystems

This has only been tested against ext4 filesystems. Other file systems may
not work out of the box.

### On-disk quota files

Linux supports two ways of storing quota data on an ext4 filesystem. They
can either be stored as a regular file (often called `aquota.user`) or as a
hidden inode. The latter can be enabled by passing `-O quota` to `mkfs.ext4`
or `tune2fs`.

`quota-exporter` should work with either schemes, but using the hidden inode is
generally more reliable and is recommended.
