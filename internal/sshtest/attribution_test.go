package sshtest

import "testing"

func TestParseContainerCgroups(t *testing.T) {
	raw := `/proc/100/cgroup:0::/system.slice/postgresql.service
/proc/200/cgroup:0::/system.slice/docker-3f1a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7f8.scope
/proc/300/cgroup:11:pids:/docker/aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899
/proc/400/cgroup:0::/system.slice/docker.service
/proc/500/cgroup:0::/system.slice/containerd.service
/proc/600/cgroup:0::/machine.slice/libpod-99887766554433221100ffeeddccbbaa99887766554433221100ffeeddccbbaa.scope
/proc/700/cgroup:0::/lxc/webserver`

	got := parseContainerCgroups(raw)

	// The engines' own daemons live in host cgroups and must not be flagged.
	for _, pid := range []int{100, 400, 500} {
		if _, ok := got[pid]; ok {
			t.Errorf("pid %d flagged as containerized, want host", pid)
		}
	}
	if got[200] != "3f1a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7f8" {
		t.Errorf("cgroup v2 docker id = %q", got[200])
	}
	if got[300] != "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899" {
		t.Errorf("cgroup v1 docker id = %q", got[300])
	}
	if got[600] != "99887766554433221100ffeeddccbbaa99887766554433221100ffeeddccbbaa" {
		t.Errorf("podman id = %q", got[600])
	}
	if id, ok := got[700]; !ok || id != "" {
		t.Errorf("lxc pid = (%q, %v), want ('', true)", id, ok)
	}
}

func catalogSpec(t *testing.T, name string) serviceSpec {
	t.Helper()
	for _, spec := range serviceCatalog {
		if spec.Name == name {
			return spec
		}
	}
	t.Fatalf("no catalog entry %q", name)
	return serviceSpec{}
}

// A postgres running only inside a container must not be reported as a host
// service, even though the host's `ps` and `ss` both see it.
func TestDetectService_ContainerOnly(t *testing.T) {
	const cid = "3f1a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7f8"
	containers := []ContainerInfo{{ID: "3f1a2b3c4d5e", Name: "db", Image: "postgres:16", Ports: "0.0.0.0:5432->5432/tcp"}}
	inv := &hostInventory{
		UnitFiles:     map[string]string{},
		LoadedUnits:   map[string]string{},
		Packages:      map[string]string{},
		Procs:         []procEntry{{pid: 200, user: "postgres", comm: "postgres"}},
		PortsByProc:   map[string][]int{},
		ListenPorts:   map[int]bool{5432: true},
		ContainerPIDs: map[int]string{200: cid},
	}

	svc, ok := detectService(catalogSpec(t, "postgresql"), inv, containers,
		dockerHostPortMap(containers), map[int]bool{}, map[int]bool{})
	if !ok {
		t.Fatal("postgres not detected at all")
	}
	if svc.HostRunning {
		t.Errorf("HostRunning = true for a container-only postgres: %+v", svc)
	}
	if svc.ContainerID != "3f1a2b3c4d5e" || svc.ContainerImage != "postgres:16" {
		t.Errorf("container binding = %q/%q, want the `docker ps` row", svc.ContainerID, svc.ContainerImage)
	}
	if svc.PID != 0 {
		t.Errorf("PID = %d, want 0 (the process belongs to the container)", svc.PID)
	}
	if len(svc.HostPorts) != 0 {
		t.Errorf("HostPorts = %v, want none (5432 is published by the container)", svc.HostPorts)
	}
}

// A host running a native postgres *and* a postgres container reports both:
// HostRunning for the systemd instance, plus the container binding.
func TestDetectService_NativeAndContainer(t *testing.T) {
	const cid = "3f1a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7f8"
	containers := []ContainerInfo{{ID: "3f1a2b3c4d5e", Name: "db", Image: "postgres:16", Ports: "0.0.0.0:15432->5432/tcp"}}
	inv := &hostInventory{
		UnitFiles:   map[string]string{"postgresql.service": "enabled"},
		LoadedUnits: map[string]string{"postgresql.service": "active"},
		Packages:    map[string]string{"postgresql": "15.4"},
		Procs: []procEntry{
			{pid: 100, user: "postgres", comm: "postgres"}, // host instance
			{pid: 200, user: "postgres", comm: "postgres"}, // containerized
		},
		PortsByProc:   map[string][]int{},
		ListenPorts:   map[int]bool{5432: true, 15432: true},
		ContainerPIDs: map[int]string{200: cid},
	}

	svc, ok := detectService(catalogSpec(t, "postgresql"), inv, containers,
		dockerHostPortMap(containers), map[int]bool{}, map[int]bool{})
	if !ok {
		t.Fatal("postgres not detected")
	}
	if !svc.HostRunning {
		t.Errorf("HostRunning = false, want true (active unit + non-containerized pid): %+v", svc)
	}
	if svc.PID != 100 {
		t.Errorf("PID = %d, want 100 (the host process, not the container's)", svc.PID)
	}
	if svc.ContainerID != "3f1a2b3c4d5e" {
		t.Errorf("ContainerID = %q, want the container's short id", svc.ContainerID)
	}
	if len(svc.HostPorts) != 1 || svc.HostPorts[0] != 5432 {
		t.Errorf("HostPorts = %v, want [5432]", svc.HostPorts)
	}
	if svc.Version != "15.4" {
		t.Errorf("Version = %q, want the host package version", svc.Version)
	}
}

// An installed-but-stopped service is detected for the scan panel but is not
// running on the host, so it never becomes a service row.
func TestDetectService_PackageOnlyIsNotHostRunning(t *testing.T) {
	inv := &hostInventory{
		UnitFiles:     map[string]string{"redis-server.service": "disabled"},
		LoadedUnits:   map[string]string{},
		Packages:      map[string]string{"redis-server": "7.0.15"},
		PortsByProc:   map[string][]int{},
		ListenPorts:   map[int]bool{},
		ContainerPIDs: map[int]string{},
	}
	svc, ok := detectService(catalogSpec(t, "redis"), inv, nil, map[int]*ContainerInfo{}, map[int]bool{}, map[int]bool{})
	if !ok {
		t.Fatal("redis not detected")
	}
	if svc.HostRunning {
		t.Errorf("HostRunning = true for an installed-but-stopped redis: %+v", svc)
	}
}
