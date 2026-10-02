package api

import (
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestDerivedProjectEdges(t *testing.T) {
	p := int64(1)
	services := []models.Service{{ID: 10, ProjectID: &p}, {ID: 11, ProjectID: &p}, {ID: 12}}
	links := store.GraphLinks{
		ServiceHost: []store.LinkPair{{From: 10, To: 100}, {From: 11, To: 100}, {From: 11, To: 101}, {From: 12, To: 102}},
		ServiceDNS:  []store.LinkPair{{From: 10, To: 200}},
		ProjectHost: []store.LinkPair{{From: 1, To: 101}}, // direct: no derived twin
	}
	hosts := map[int64]string{100: "host-100", 101: "host-101", 102: "host-102"}
	dns := map[int64]string{200: "dns-200"}
	projects := map[int64]string{1: "project-1"}
	got := derivedProjectEdges(services, links, hosts, dns, projects)
	want := map[string]bool{"host-100>project-1": true, "dns-200>project-1": true}
	if len(got) != len(want) {
		t.Fatalf("edges = %+v, want %v", got, want)
	}
	for _, e := range got {
		if !want[e.Source+">"+e.Target] || !e.Derived || e.Label != "via service" {
			t.Errorf("unexpected edge %+v", e)
		}
	}
}
