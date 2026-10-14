package service

import (
	"regexp"
	"sort"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/coolify"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/sentinel"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// serviceKey is a scan-discovered service's container identity
// ("airflow-apiserver-<uuid>", or "<uuid>" for a plain app) and its Coolify
// resource uuid; "" when the service isn't a container. A host-kind service
// (systemd/process) is never a container, even when its catalog name
// ("postgresql") happens to equal one's.
func serviceKey(svc models.Service) (key, uuid string) {
	name := svc.ContainerName
	if name == "" && svc.DiscoveryKind == "container" {
		name = svc.DiscoveryKey
	}
	if name == "" {
		return "", ""
	}
	key, uuid = coolify.ContainerKey(strings.ToLower(name))
	if uuid == "" {
		uuid = svc.CoolifyResourceUUID
	}
	return key, uuid
}

// routerHash is the 4-hex suffix Coolify adds to a compose router name
// (`<uuid>-airflow-apiserver-0c35`).
var routerHash = regexp.MustCompile(`-[0-9a-f]{4}$`)

// trafficKeys picks the Sentinel app keys that belong to svc. Sentinel names
// an app after its Traefik router: `<uuid>` (plain app) or `<uuid>-<member>`
// with an optional hash (compose app / service stack member).
func trafficKeys(svc models.Service, keys []string) []string {
	key, uuid := serviceKey(svc)
	if uuid == "" {
		return nil
	}
	member := ""
	if key != uuid {
		member = strings.TrimSuffix(key, "-"+uuid)
	}
	var out []string
	for _, k := range keys {
		switch {
		case member == "":
			if k == uuid || routerHash.ReplaceAllString(k, "") == uuid {
				out = append(out, k)
			}
		case strings.HasPrefix(k, uuid+"-"):
			rest := strings.TrimPrefix(k, uuid+"-")
			if rest == member || routerHash.ReplaceAllString(rest, "") == member {
				out = append(out, k)
			}
		}
	}
	return out
}

// containerFor finds svc's container among Sentinel's live ones by the
// redeploy-stable key, so a deploy since the last scan still matches.
func containerFor(cs []sentinel.Container, svc models.Service) *sentinel.Container {
	key, _ := serviceKey(svc)
	if key == "" {
		return nil
	}
	var found *sentinel.Container
	for i := range cs {
		if k, _ := coolify.ContainerKey(cs[i].ID); k != key {
			continue
		}
		if found == nil || cs[i].Status.State == "running" {
			found = &cs[i]
		}
	}
	return found
}

// publishesPort: docker's port list has a host binding ("0.0.0.0:9000->9000/tcp").
func publishesPort(ports string) bool { return strings.Contains(ports, "->") }

// sumOverview adds several app keys' overviews.
// ponytail: latencies take the max (a percentile can't be summed); merging
// t-digests would need Sentinel's resource scope per member.
func sumOverview(os []*sentinel.Overview) sentinel.Overview {
	var t sentinel.Overview
	for _, o := range os {
		t.Requests += o.Requests
		t.BytesIn += o.BytesIn
		t.BytesOut += o.BytesOut
		t.Status.S2xx += o.Status.S2xx
		t.Status.S3xx += o.Status.S3xx
		t.Status.S4xx += o.Status.S4xx
		t.Status.S5xx += o.Status.S5xx
		t.UniqueVisitors += o.UniqueVisitors
		t.Latency.P50 = max(t.Latency.P50, o.Latency.P50)
		t.Latency.P95 = max(t.Latency.P95, o.Latency.P95)
		t.Latency.P99 = max(t.Latency.P99, o.Latency.P99)
	}
	return t
}

// mergeSeries adds series bucket by bucket.
func mergeSeries(ss [][]sentinel.Bucket) []sentinel.Bucket {
	if len(ss) == 1 {
		return ss[0]
	}
	by := map[int64]*sentinel.Bucket{}
	for _, s := range ss {
		for _, b := range s {
			m, ok := by[b.Bucket]
			if !ok {
				c := b
				by[b.Bucket] = &c
				continue
			}
			m.Requests += b.Requests
			m.S2xx += b.S2xx
			m.S3xx += b.S3xx
			m.S4xx += b.S4xx
			m.S5xx += b.S5xx
			m.UniqueVisitors += b.UniqueVisitors
			m.P95 = max(m.P95, b.P95)
		}
	}
	out := make([]sentinel.Bucket, 0, len(by))
	for _, b := range by {
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Bucket < out[j].Bucket })
	return out
}

// topPaths concatenates paths of several keys, busiest first.
func topPaths(ps [][]sentinel.Path, limit int) []sentinel.Path {
	var out []sentinel.Path
	for _, p := range ps {
		out = append(out, p...)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Requests > out[j].Requests })
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// mergeBreakdown adds rows with the same value, busiest first.
func mergeBreakdown(rs [][]sentinel.BreakdownRow, limit int) []sentinel.BreakdownRow {
	by := map[string]*sentinel.BreakdownRow{}
	var order []string
	for _, r := range rs {
		for _, row := range r {
			m, ok := by[row.Value]
			if !ok {
				c := row
				by[row.Value] = &c
				order = append(order, row.Value)
				continue
			}
			m.Requests += row.Requests
			m.BytesOut += row.BytesOut
		}
	}
	out := make([]sentinel.BreakdownRow, 0, len(order))
	for _, v := range order {
		out = append(out, *by[v])
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Requests > out[j].Requests })
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}
