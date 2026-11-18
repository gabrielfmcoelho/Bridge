package sshtest

import (
	"errors"
	"strings"
	"testing"
)

// SAPASP-AVALIE04: `docker ps` answered, `docker stats` lost its session.
// The container list is still known; only the CPU/MEM columns are missing.
func TestDockerStatsFailureKeepsContainerListKnown(t *testing.T) {
	warn := map[string]bool{}
	ps, stats := dockerPsAndStats(func(cmd string) (string, error) {
		if strings.Contains(cmd, "docker stats") {
			return "", errors.New("new session: ssh: rejected: connect failed (open failed)")
		}
		return "4b82b86b1b62\tform-totem-nextjs\timg\tUp 13 months\t0.0.0.0:3000->3000/tcp", nil
	}, warn)
	if ps == "" || stats != "" || warn["docker_unreadable"] {
		t.Fatalf("ps=%q stats=%q warn=%v", ps, stats, warn)
	}
}

func TestDockerPsFailureMarksContainerListUnknown(t *testing.T) {
	warn := map[string]bool{}
	dockerPsAndStats(func(string) (string, error) { return "", errors.New("ssh cmd timeout") }, warn)
	if !warn["docker_unreadable"] {
		t.Fatal("a failed docker ps must leave the container list unknown")
	}
}

func grafanaOn3000(published map[int]*ContainerInfo) (Agent, bool) {
	agents, _ := captureAgents(&hostInventory{ListenPorts: map[int]bool{3000: true}}, published)
	for _, a := range agents {
		if a.Name == "grafana-server" {
			return a, true
		}
	}
	return Agent{}, false
}

// A Next.js container publishing :3000 is not Grafana.
func TestSignaturePortOfAnotherContainerIsNotAnAgent(t *testing.T) {
	c := &ContainerInfo{Name: "form-totem-nextjs", Image: "front-avalie-servicos-totem-form-totem-nextjs"}
	if a, ok := grafanaOn3000(map[int]*ContainerInfo{3000: c}); ok {
		t.Fatalf("matched %+v", a)
	}
}

func TestSignaturePortOfTheAgentsOwnContainerStillMatches(t *testing.T) {
	c := &ContainerInfo{Name: "monitoring", Image: "grafana/grafana:10.4"}
	if _, ok := grafanaOn3000(map[int]*ContainerInfo{3000: c}); !ok {
		t.Fatal("grafana container on :3000 not matched")
	}
}

// A port alone says nothing about files on the host.
func TestPortOnlyAgentClaimsNoConfigPath(t *testing.T) {
	a, ok := grafanaOn3000(nil)
	if !ok || a.ConfigPath != "" {
		t.Fatalf("ok=%v config_path=%q", ok, a.ConfigPath)
	}
}
