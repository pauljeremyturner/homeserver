# Patched amdgpu for the t530

The t530's passive DP->HDMI adapter never raises HPD, so stock amdgpu calls
every DP port disconnected and sends no picture (Windows and the firmware
cope; forcing the port with `video=DP-1:e` only drives a virtual sink).
`amdgpu-hpd-ddc-fallback.patch` makes DC treat a DP port with HPD low but a
valid EDID on DDC as connected, so it takes the passive-dongle (DVI/HDMI)
path. The screen is only detected at boot: no hotplug.

On the t530 (kernel 6.19.10-300.fc44, Secure Boot off since the module is
unsigned) the module is `/lib/modules/6.19.10-300.fc44.x86_64/updates/amdgpu.ko`,
it's in the initramfs (`dracut -f`, or the stock one loads first), and the
kernel packages are pinned with `dnf versionlock` so updates can't replace it.

## Rebuilding (only if the kernel ever changes)

Build just the module, on a faster box, in a container matching the kernel:

1. From koji.fedoraproject.org/packages/kernel/<ver>/<rel>/, fetch the
   `.src.rpm` and `x86_64/kernel-devel-*.rpm` into `dl/`.
2. `rpm2cpio dl/kernel-*.src.rpm | cpio -idm`, untar `linux-*.tar.xz`,
   `patch -p1 < patch-*-redhat.patch`, then `patch -p1 <` this patch.
3. In `fedora:<rel>` with `dl/` at /dl and the tree at /src:
   ```
   dnf install /dl/kernel-devel-*.rpm gcc make elfutils-libelf-devel bc flex bison openssl-devel dwarves
   K=/usr/src/kernels/<ver>-<rel>.x86_64
   ln -s /src/drivers/gpu/drm/amd $K/drivers/gpu/drm/amd   # amdgpu_trace.h's include path
   make -C $K M=/src/drivers/gpu/drm/amd/amdgpu -j$(nproc) modules
   ```
4. `strip --strip-debug amdgpu.ko` (720 MB -> 43 MB), check `modinfo`'s
   vermagic matches, install into `updates/`, `depmod -a`, `dracut -f`.

Test without installing: boot with `modprobe.blacklist=amdgpu`, then
`insmod` it. Swapping it in over a running stock amdgpu fails the GPU's IB
test and can hang the next warm reboot.

To go back to stock: delete `updates/amdgpu.ko`, `depmod -a`, `dracut -f`,
`dnf versionlock delete kernel\*`.
