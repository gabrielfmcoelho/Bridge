package service

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/sentinel"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// App keys as Sentinel 1.0.2 reported them on the master and on Nodo F.
var liveKeys = []string{
	"a1rwyyxk4rby396m4ooq4ubu",
	"m0cwcowwg0gsk8kscggwc4w0-otel-collector-ae0a",
	"m88c84c4ww48kgwocsos0gsk-outline",
	"rtb37u69bbkepzukr9vclrd7-airflow-apiserver-0c35",
	"rtb37u69bbkepzukr9vclrd7-flower-608f",
}

func TestTrafficKeys(t *testing.T) {
	cases := []struct {
		svc  models.Service
		want []string
	}{
		// compose app member, scanned before Coolify 4.4's timestamp format
		{models.Service{ContainerName: "airflow-apiserver-rtb37u69bbkepzukr9vclrd7-123344876560"}, []string{"rtb37u69bbkepzukr9vclrd7-airflow-apiserver-0c35"}},
		// … and after
		{models.Service{ContainerName: "flower-rtb37u69bbkepzukr9vclrd7-20261013T214301"}, []string{"rtb37u69bbkepzukr9vclrd7-flower-608f"}},
		// a member without traffic must not pick up its siblings'
		{models.Service{ContainerName: "airflow-worker-rtb37u69bbkepzukr9vclrd7-123345134951"}, nil},
		// plain application
		{models.Service{ContainerName: "a1rwyyxk4rby396m4ooq4ubu-143303502430"}, []string{"a1rwyyxk4rby396m4ooq4ubu"}},
		// service stack members, with and without the router hash
		{models.Service{ContainerName: "outline-m88c84c4ww48kgwocsos0gsk-111111111111"}, []string{"m88c84c4ww48kgwocsos0gsk-outline"}},
		{models.Service{ContainerName: "otel-collector-m0cwcowwg0gsk8kscggwc4w0-111111111111"}, []string{"m0cwcowwg0gsk8kscggwc4w0-otel-collector-ae0a"}},
		// not a Coolify container
		{models.Service{ContainerName: "postgres-15"}, nil},
	}
	for _, c := range cases {
		if got := trafficKeys(c.svc, liveKeys); !reflect.DeepEqual(got, c.want) {
			t.Errorf("trafficKeys(%s) = %v, want %v", c.svc.ContainerName, got, c.want)
		}
	}
}

func TestContainerFor(t *testing.T) {
	var cs []sentinel.Container
	if err := json.Unmarshal([]byte(`[
		{"id":"flower-rtb37u69bbkepzukr9vclrd7-20261009t135443","status":{"state":"exited","health":"unknown","restartCount":0}},
		{"id":"flower-rtb37u69bbkepzukr9vclrd7-20261013t214301","cpu":{"time":"1","percent":"0.08"},"status":{"state":"running","health":"healthy","restartCount":2}},
		{"id":"redis-njceio8292x8wcpuun3cprfi-20261013t120548","status":{"state":"running","health":"healthy","restartCount":0}}
	]`), &cs); err != nil {
		t.Fatal(err)
	}
	// The scan saw an older deploy; the running container still matches.
	c := containerFor(cs, models.Service{ContainerName: "flower-rtb37u69bbkepzukr9vclrd7-123345302783"})
	if c == nil || c.ID != "flower-rtb37u69bbkepzukr9vclrd7-20261013t214301" || c.Status.RestartCount != 2 {
		t.Fatalf("containerFor = %+v", c)
	}
	if containerFor(cs, models.Service{ContainerName: "postgresql-njceio8292x8wcpuun3cprfi-1"}) != nil {
		t.Fatal("different member must not match")
	}
	if containerFor(cs, models.Service{}) != nil {
		t.Fatal("manual service must not match")
	}
	// A host-kind service (systemd postgres) must not take a container named like its catalog entry.
	var stale sentinel.Container
	_ = json.Unmarshal([]byte(`{"id":"postgresql","status":{"state":"exited","health":"unhealthy","restartCount":0}}`), &stale)
	cs = append(cs, stale)
	if containerFor(cs, models.Service{DiscoveryKind: "host", DiscoveryKey: "postgresql"}) != nil {
		t.Fatal("host-kind service must not match a container")
	}
}

func TestMergeTraffic(t *testing.T) {
	a := &sentinel.Overview{Requests: 10, Status: sentinel.StatusCounts{S2xx: 9, S5xx: 1}}
	a.Latency.P95 = 100
	b := &sentinel.Overview{Requests: 5, Status: sentinel.StatusCounts{S2xx: 5}}
	b.Latency.P95 = 300
	o := sumOverview([]*sentinel.Overview{a, b})
	if o.Requests != 15 || o.Status.S5xx != 1 || o.Latency.P95 != 300 {
		t.Fatalf("sumOverview = %+v", o)
	}
	s := mergeSeries([][]sentinel.Bucket{
		{{Bucket: 2, Requests: 1}, {Bucket: 1, Requests: 2}},
		{{Bucket: 1, Requests: 3, P95: 9}},
	})
	if len(s) != 2 || s[0].Bucket != 1 || s[0].Requests != 5 || s[0].P95 != 9 {
		t.Fatalf("mergeSeries = %+v", s)
	}
	p := topPaths([][]sentinel.Path{{{Path: "/a", Requests: 1}}, {{Path: "/b", Requests: 7}, {Path: "/c", Requests: 3}}}, 2)
	if len(p) != 2 || p[0].Path != "/b" || p[1].Path != "/c" {
		t.Fatalf("topPaths = %+v", p)
	}
	br := mergeBreakdown([][]sentinel.BreakdownRow{{{Value: "200", Requests: 4}}, {{Value: "500", Requests: 1}, {Value: "200", Requests: 2}}}, 10)
	if len(br) != 2 || br[0].Value != "200" || br[0].Requests != 6 {
		t.Fatalf("mergeBreakdown = %+v", br)
	}
	if !publishesPort("0.0.0.0:9000-9001->9000-9001/tcp") || publishesPort("8080/tcp") {
		t.Fatal("publishesPort")
	}
}
