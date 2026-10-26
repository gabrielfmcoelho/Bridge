package sshtest

import (
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// Filesystem is one mounted filesystem from df. Network ones (NFS, CIFS…) are
// listed but never counted in capacity or the disk % (another server's disk);
// a network mount that didn't answer in time has Size 0.
type Filesystem struct {
	Mount      string  `json:"mount"`
	Device     string  `json:"device"`
	FSType     string  `json:"fstype"`
	SizeBytes  int64   `json:"size_bytes"`
	UsedBytes  int64   `json:"used_bytes"`
	AvailBytes int64   `json:"avail_bytes"`
	UsedPct    float64 `json:"used_pct"`
	Network    bool    `json:"network,omitempty"`
}

// BlockDisk is one raw disk from lsblk. Unused: no partition, LVM member or
// filesystem on it at all.
type BlockDisk struct {
	Name      string `json:"name"`
	SizeBytes int64  `json:"size_bytes"`
	Unused    bool   `json:"unused,omitempty"`
}

// VolumeGroup is one LVM volume group and its unallocated space.
type VolumeGroup struct {
	Name      string `json:"name"`
	SizeBytes int64  `json:"size_bytes"`
	FreeBytes int64  `json:"free_bytes"`
}

const storageTimeout = 30 * time.Second

var networkFSTypes = map[string]bool{
	"nfs": true, "nfs4": true, "cifs": true, "smbfs": true, "smb3": true,
	"fuse.sshfs": true, "glusterfs": true, "ceph": true, "fuse.glusterfs": true, "9p": true,
}

// storageScript: @@df local filesystems, @@net network ones (short timeout:
// a dead NFS server would hang df), @@mounts network mounts from /proc
// (listed even when df gave up), @@lsblk raw disks, @@vgs LVM free space.
// SUDO=1 retries vgs with sudo -n (it needs root); the root run sets nothing.
func storageScript(asRoot bool) string {
	sudoVgs := ""
	if !asRoot {
		sudoVgs = ` || sudo -n $T vgs --noheadings --units b --nosuffix -o vg_name,vg_size,vg_free 2>/dev/null`
	}
	var netTypes []string
	for t := range networkFSTypes {
		netTypes = append(netTypes, "-t "+t)
	}
	sort.Strings(netTypes)
	return `export PATH="$PATH:/usr/local/sbin:/usr/sbin:/sbin"
T=""; command -v timeout >/dev/null 2>&1 && T="timeout 10"
echo @@df; $T df -lPTB1 2>/dev/null
echo @@net; if [ -n "$T" ]; then timeout 5 df -PTB1 ` + strings.Join(netTypes, " ") + ` 2>/dev/null; fi
echo @@mounts; awk '{print $1, $2, $3}' /proc/mounts 2>/dev/null
echo @@lsblk; $T lsblk -b -P -o NAME,SIZE,TYPE,FSTYPE,MOUNTPOINT,PKNAME 2>/dev/null
echo @@vgs; $T vgs --noheadings --units b --nosuffix -o vg_name,vg_size,vg_free 2>/dev/null` + sudoVgs + `
`
}

// collectStorage runs the storage probe — as root with the stored password
// (vgs needs it), else unprivileged.
func collectStorage(client *ssh.Client, sudoPassword string) string {
	if sudoPassword != "" {
		out, err := runSudoCmd(client, sudoPassword, storageScript(true))
		if strings.Contains(out, "@@df") {
			return out
		}
		log.Printf("[sshscan] storage as root failed, falling back to unprivileged: %v", err)
	}
	out, _ := runCmdWithDeadline(client, encodedScript(storageScript(false), "sh"), storageTimeout)
	return out
}

// Storage is the parsed probe.
type Storage struct {
	Filesystems  []Filesystem
	Disks        []BlockDisk
	VolumeGroups []VolumeGroup
}

func parseStorage(raw string) Storage {
	sec := map[string][]string{}
	cur := ""
	for _, line := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "@@") && !strings.Contains(t, " ") {
			cur = strings.TrimPrefix(t, "@@")
			continue
		}
		if cur != "" && t != "" {
			sec[cur] = append(sec[cur], t)
		}
	}
	var s Storage
	s.Filesystems = localFilesystems(sec["df"])
	s.Filesystems = append(s.Filesystems, networkFilesystems(sec["net"], sec["mounts"])...)
	s.Disks = parseLsblk(sec["lsblk"])
	s.VolumeGroups = parseVgs(sec["vgs"])
	return s
}

var skipFSTypes = map[string]bool{
	"tmpfs": true, "devtmpfs": true, "overlay": true, "squashfs": true, "efivarfs": true, "ramfs": true,
	"nsfs": true, "autofs": true, "proc": true, "sysfs": true, "cgroup": true, "cgroup2": true,
	"fuse.snapfuse": true, "fuse.lxcfs": true, "iso9660": true, "udf": true, "tracefs": true, "debugfs": true,
}

var skipMountPrefixes = []string{"/var/lib/docker/", "/var/lib/kubelet/", "/var/lib/containers/", "/run/", "/snap/", "/sys/", "/proc/", "/dev/"}

func skipMount(mount string) bool {
	for _, p := range skipMountPrefixes {
		if strings.HasPrefix(mount+"/", p) {
			return true
		}
	}
	return false
}

// parseDfLine reads one `df -PTB1` row: device type size used avail pct mount…
func parseDfLine(line string) (Filesystem, bool) {
	f := strings.Fields(line)
	if len(f) < 7 || f[0] == "Filesystem" {
		return Filesystem{}, false
	}
	size, err1 := strconv.ParseInt(f[2], 10, 64)
	used, err2 := strconv.ParseInt(f[3], 10, 64)
	avail, err3 := strconv.ParseInt(f[4], 10, 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return Filesystem{}, false
	}
	pct, err := strconv.ParseFloat(strings.TrimSuffix(f[5], "%"), 64)
	if err != nil && used+avail > 0 {
		pct = float64(used) * 100 / float64(used+avail)
	}
	return Filesystem{Device: f[0], FSType: f[1], SizeBytes: size, UsedBytes: used, AvailBytes: avail,
		UsedPct: pct, Mount: strings.Join(f[6:], " ")}, true
}

// localFilesystems keeps real local filesystems: no pseudo/overlay/snap types,
// no loop devices, no container-runtime or system mounts, no empty ones. A
// device mounted twice (bind mount) is kept once, at its shortest path.
func localFilesystems(lines []string) []Filesystem {
	byDevice := map[string]Filesystem{}
	for _, l := range lines {
		fs, ok := parseDfLine(l)
		if !ok || fs.SizeBytes == 0 || skipFSTypes[fs.FSType] || networkFSTypes[fs.FSType] ||
			strings.HasPrefix(fs.Device, "/dev/loop") || skipMount(fs.Mount) {
			continue
		}
		if prev, seen := byDevice[fs.Device]; !seen || len(fs.Mount) < len(prev.Mount) {
			byDevice[fs.Device] = fs
		}
	}
	out := make([]Filesystem, 0, len(byDevice))
	for _, fs := range byDevice {
		out = append(out, fs)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Mount < out[j].Mount })
	return out
}

// networkFilesystems: the ones df answered for (with sizes), plus network
// mounts from /proc/mounts it didn't reach in time (size 0).
func networkFilesystems(dfLines, mountLines []string) []Filesystem {
	var out []Filesystem
	seen := map[string]bool{}
	for _, l := range dfLines {
		if fs, ok := parseDfLine(l); ok && networkFSTypes[fs.FSType] && !seen[fs.Mount] {
			fs.Network = true
			out = append(out, fs)
			seen[fs.Mount] = true
		}
	}
	for _, l := range mountLines {
		f := strings.Fields(l)
		if len(f) == 3 && networkFSTypes[f[2]] && !seen[f[1]] {
			out = append(out, Filesystem{Device: f[0], Mount: f[1], FSType: f[2], Network: true})
			seen[f[1]] = true
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Mount < out[j].Mount })
	return out
}

var lsblkPair = regexp.MustCompile(`([A-Z:]+)="([^"]*)"`)

// parseLsblk reads `lsblk -b -P` rows into raw disks; a disk is Unused when
// nothing sits on it (no child row, no filesystem, no mount).
func parseLsblk(lines []string) []BlockDisk {
	type row struct{ name, size, typ, fstype, mount, parent string }
	var rows []row
	for _, l := range lines {
		kv := map[string]string{}
		for _, m := range lsblkPair.FindAllStringSubmatch(l, -1) {
			kv[m[1]] = m[2]
		}
		rows = append(rows, row{kv["NAME"], kv["SIZE"], kv["TYPE"], kv["FSTYPE"], kv["MOUNTPOINT"], kv["PKNAME"]})
	}
	hasChild := map[string]bool{}
	for _, r := range rows {
		if r.parent != "" {
			hasChild[r.parent] = true
		}
	}
	var out []BlockDisk
	for _, r := range rows {
		if r.typ != "disk" || strings.HasPrefix(r.name, "loop") || strings.HasPrefix(r.name, "sr") || strings.HasPrefix(r.name, "zram") {
			continue
		}
		size, err := strconv.ParseInt(r.size, 10, 64)
		if err != nil || size == 0 {
			continue
		}
		out = append(out, BlockDisk{Name: r.name, SizeBytes: size, Unused: !hasChild[r.name] && r.fstype == "" && r.mount == ""})
	}
	return out
}

// parseVgs reads `vgs --noheadings --units b --nosuffix -o vg_name,vg_size,vg_free`.
func parseVgs(lines []string) []VolumeGroup {
	var out []VolumeGroup
	for _, l := range lines {
		f := strings.Fields(l)
		if len(f) != 3 {
			continue
		}
		size, err1 := strconv.ParseInt(f[1], 10, 64)
		free, err2 := strconv.ParseInt(f[2], 10, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		out = append(out, VolumeGroup{Name: f[0], SizeBytes: size, FreeBytes: free})
	}
	return out
}

// StorageTotals are the host-level numbers: local filesystems only.
// Fullest is the mount with the highest use — the host's disk %.
func StorageTotals(fss []Filesystem) (size, used int64, fullest Filesystem, ok bool) {
	for _, fs := range fss {
		if fs.Network {
			continue
		}
		ok = true
		size += fs.SizeBytes
		used += fs.UsedBytes
		if fs.UsedPct > fullest.UsedPct || fullest.Mount == "" {
			fullest = fs
		}
	}
	return
}

// humanBytes formats like `df -h` (1024-based, one decimal under 10): "150G", "1.6T".
func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	v := float64(b) / float64(div)
	suffix := string("KMGTPE"[exp])
	if v < 10 {
		return strconv.FormatFloat(v, 'f', 1, 64) + suffix
	}
	return fmt.Sprintf("%.0f%s", v, suffix)
}
