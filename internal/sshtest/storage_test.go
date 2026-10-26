package sshtest

import (
	"os/exec"
	"reflect"
	"testing"
)

// gestor's real df/lsblk output, plus a bind mount, a docker overlay, a snap,
// an NFS mount that answered and one that didn't, an unused disk and LVM.
const storageFixture = `@@df
Filesystem                       Type      1-blocks         Used    Available Capacity Mounted on
tmpfs                            tmpfs   1672822784      1847296   1670975488       1% /run
/dev/mapper/ubuntu--vg-lv--raiz  xfs   160982630400 100536287232  60446343168      63% /
tmpfs                            tmpfs   8364113920            0   8364113920       0% /dev/shm
/dev/sda2                        ext4    2040373248    273543168   1642680320      15% /boot
/dev/sdb                         ext4  632984600576        28672 600755539968       1% /mnt/backup
/dev/sda1                        vfat    1124995072      6381568   1118613504       1% /boot/efi
/dev/mapper/ubuntu--vg-lv--banco xfs   900370821120 157709062144 742661758976      18% /banco
/dev/sdb                         ext4  632984600576        28672 600755539968       1% /srv/backup bind
overlay                          overlay 160982630400 100536287232 60446343168      63% /var/lib/docker/overlay2/abc/merged
/dev/loop3                       squashfs    77594624     77594624            0     100% /snap/core22/2437
@@net
Filesystem                 Type 1-blocks Used Available Capacity Mounted on
10.0.0.9:/export/arquivos  nfs4 1099511627776 989560464998 109951162778 91% /mnt/arquivos
@@mounts
/dev/mapper/ubuntu--vg-lv--raiz / xfs
10.0.0.9:/export/arquivos /mnt/arquivos nfs4
//fs01/share /mnt/share cifs
@@lsblk
NAME="sda" SIZE="1073741824000" TYPE="disk" FSTYPE="" MOUNTPOINT="" PKNAME=""
NAME="sda1" SIZE="1127219200" TYPE="part" FSTYPE="vfat" MOUNTPOINT="/boot/efi" PKNAME="sda"
NAME="sda3" SIZE="1070465024000" TYPE="part" FSTYPE="LVM2_member" MOUNTPOINT="" PKNAME="sda"
NAME="ubuntu--vg-lv--raiz" SIZE="161061273600" TYPE="lvm" FSTYPE="xfs" MOUNTPOINT="/" PKNAME="sda3"
NAME="sdb" SIZE="644245094400" TYPE="disk" FSTYPE="ext4" MOUNTPOINT="/mnt/backup" PKNAME=""
NAME="sdc" SIZE="107374182400" TYPE="disk" FSTYPE="" MOUNTPOINT="" PKNAME=""
NAME="sr0" SIZE="1073741312" TYPE="rom" FSTYPE="" MOUNTPOINT="" PKNAME=""
NAME="loop3" SIZE="77594624" TYPE="loop" FSTYPE="squashfs" MOUNTPOINT="/snap/core22/2437" PKNAME=""
@@vgs
  ubuntu-vg 1070465024000 0
  dados     214748364800 53687091200
`

func TestParseStorage(t *testing.T) {
	s := parseStorage(storageFixture)

	var mounts []string
	for _, fs := range s.Filesystems {
		mounts = append(mounts, fs.Mount)
	}
	want := []string{"/", "/banco", "/boot", "/boot/efi", "/mnt/backup", "/mnt/arquivos", "/mnt/share"}
	if !reflect.DeepEqual(mounts, want) {
		t.Fatalf("mounts = %v, want %v (no tmpfs/overlay/snap, bind mount once, network last)", mounts, want)
	}
	if fs := s.Filesystems[1]; fs.Device != "/dev/mapper/ubuntu--vg-lv--banco" || fs.FSType != "xfs" ||
		fs.SizeBytes != 900370821120 || fs.UsedBytes != 157709062144 || fs.UsedPct != 18 {
		t.Errorf("/banco = %+v", fs)
	}
	if fs := s.Filesystems[5]; !fs.Network || fs.SizeBytes != 1099511627776 || fs.UsedPct != 91 {
		t.Errorf("answered NFS = %+v", fs)
	}
	if fs := s.Filesystems[6]; !fs.Network || fs.SizeBytes != 0 || fs.FSType != "cifs" {
		t.Errorf("unanswered CIFS = %+v", fs)
	}

	wantDisks := []BlockDisk{
		{Name: "sda", SizeBytes: 1073741824000},
		{Name: "sdb", SizeBytes: 644245094400},
		{Name: "sdc", SizeBytes: 107374182400, Unused: true},
	}
	if !reflect.DeepEqual(s.Disks, wantDisks) {
		t.Errorf("disks = %+v, want %+v", s.Disks, wantDisks)
	}
	wantVG := []VolumeGroup{{"ubuntu-vg", 1070465024000, 0}, {"dados", 214748364800, 53687091200}}
	if !reflect.DeepEqual(s.VolumeGroups, wantVG) {
		t.Errorf("vgs = %+v", s.VolumeGroups)
	}
}

// Capacity counts local filesystems only; the disk % is the fullest local
// one — the 91% NFS share never wins.
func TestStorageTotals(t *testing.T) {
	size, used, fullest, ok := StorageTotals(parseStorage(storageFixture).Filesystems)
	wantSize := int64(160982630400 + 900370821120 + 2040373248 + 1124995072 + 632984600576)
	wantUsed := int64(100536287232 + 157709062144 + 273543168 + 6381568 + 28672)
	if !ok || size != wantSize || used != wantUsed || fullest.Mount != "/" || fullest.UsedPct != 63 {
		t.Fatalf("totals = %d %d %+v %v", size, used, fullest, ok)
	}
	if humanBytes(size) != "1.5T" || humanBytes(used) != "241G" {
		t.Fatalf("human = %s / %s", humanBytes(size), humanBytes(used))
	}
	if _, _, _, ok := StorageTotals([]Filesystem{{Mount: "/mnt/x", Network: true, UsedPct: 99}}); ok {
		t.Fatal("network only should leave the / fallback in place")
	}
}

func TestHumanBytes(t *testing.T) {
	for in, want := range map[int64]string{512: "512B", 1536: "1.5K", 161061273600: "150G", 1073741824000: "1000G", 1649267441664: "1.5T"} {
		if got := humanBytes(in); got != want {
			t.Errorf("humanBytes(%d) = %s, want %s", in, got, want)
		}
	}
}

func TestStorageScript(t *testing.T) {
	for _, asRoot := range []bool{false, true} {
		if out, err := exec.Command("sh", "-n", "-c", storageScript(asRoot)).CombinedOutput(); err != nil {
			t.Fatalf("root=%v sh -n: %v %s", asRoot, err, out)
		}
	}
}
