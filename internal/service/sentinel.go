package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/sentinel"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// Why Sentinel can't be read for a host. Shown as-is by the UI (i18n keys
// sentinel.reason.<reason>) and stored as the host snapshot's state.
const (
	SentinelOK                = "ok"
	SentinelReasonDisabled    = "disabled"         // Bridge's Sentinel integration is off
	SentinelReasonCoolifyOff  = "coolify_off"      // the Coolify integration isn't set up
	SentinelReasonNotLinked   = "not_linked"       // host has no Coolify server
	SentinelReasonSSHConfig   = "ssh_config"       // host has no usable SSH login
	SentinelReasonSSHFailed   = "ssh_failed"       // SSH connection failed
	SentinelReasonNoDocker    = "no_docker_access" // SSH user can't run docker
	SentinelReasonNoSentinel  = "no_sentinel"      // no coolify-sentinel container
	SentinelReasonNoForward   = "no_tcp_forwarding"
	SentinelReasonNoToken     = "no_token" // Coolify token lacks read:sensitive
	SentinelReasonMetricsOff  = "metrics_off"
	SentinelReasonTrafficOff  = "traffic_off"
	SentinelReasonError       = "error"
	sentinelIdleTimeout       = 5 * time.Minute
	sentinelDefaultCacheTTL   = 60 * time.Second
	sentinelSyncParallelism   = 4
	sentinelHostSyncTimeout   = 90 * time.Second
	sentinelContainer         = "coolify-sentinel"
	sentinelPort              = "8888"
	sentinelInspectIPTemplate = `{{range .NetworkSettings.Networks}}{{.IPAddress}} {{end}}`
)

// SentinelUnavailable is why a read couldn't happen; handlers answer it as
// {available:false, reason} rather than an error.
type SentinelUnavailable struct {
	Reason string
	Detail string
}

func (e *SentinelUnavailable) Error() string {
	if e.Detail == "" {
		return "sentinel: " + e.Reason
	}
	return "sentinel: " + e.Reason + ": " + e.Detail
}

func unavailable(reason, detail string) error { return &SentinelUnavailable{reason, detail} }

// sentinelConn is one host's open path to Sentinel: an SSH connection kept
// alive between reads (≈10ms a request instead of a ≈150ms handshake), the
// Sentinel container address behind it, and the Coolify flags of the server.
type sentinelConn struct {
	ssh      *ssh.Client
	api      *sentinel.Client
	metrics  bool
	traffic  bool
	lastUsed time.Time
}

func (c *sentinelConn) close() {
	if c != nil && c.ssh != nil {
		c.ssh.Close()
	}
}

type sentinelSlot struct {
	mu   sync.Mutex
	conn *sentinelConn
}

type cacheEntry struct {
	val     any
	expires time.Time
}

// SentinelService reads Coolify's Sentinel through each host's SSH
// connection: live reads for detail pages (pooled + cached), and the manual
// sync that writes the per-host / per-service snapshots lists and alerts use.
type SentinelService struct {
	db   *database.DB
	repo *store.SentinelRepo

	mu    sync.Mutex
	slots map[int64]*sentinelSlot

	cmu   sync.Mutex
	cache map[string]cacheEntry

	// dial opens a host's path to Sentinel; a seam for tests.
	dial func(ctx context.Context, host *models.Host) (*sentinelConn, error)
}

// NewSentinelService builds the service.
func NewSentinelService(db *database.DB) *SentinelService {
	s := &SentinelService{
		db:    db,
		repo:  store.NewSentinelRepo(db.SQL),
		slots: map[int64]*sentinelSlot{},
		cache: map[string]cacheEntry{},
	}
	s.dial = s.dialSSH
	return s
}

// Enabled reports whether the Sentinel integration is switched on.
func (s *SentinelService) Enabled(ctx context.Context) bool {
	return store.NewAppSettingsRepo(s.db.SQL).Value(ctx, "sentinel_enabled") == "true"
}

func (s *SentinelService) cacheTTL(ctx context.Context) time.Duration {
	if n, err := strconv.Atoi(store.NewAppSettingsRepo(s.db.SQL).Value(ctx, "sentinel_cache_seconds")); err == nil && n >= 0 {
		return time.Duration(n) * time.Second
	}
	return sentinelDefaultCacheTTL
}

// cached runs fn at most once per TTL for key.
func cached[T any](ctx context.Context, s *SentinelService, key string, fn func() (T, error)) (T, error) {
	now := time.Now()
	s.cmu.Lock()
	if e, ok := s.cache[key]; ok && now.Before(e.expires) {
		s.cmu.Unlock()
		return e.val.(T), nil
	}
	s.cmu.Unlock()
	v, err := fn()
	if err != nil {
		return v, err
	}
	s.cmu.Lock()
	for k, e := range s.cache { // ponytail: sweep on write; the key space is hosts × views
		if now.After(e.expires) {
			delete(s.cache, k)
		}
	}
	s.cache[key] = cacheEntry{v, now.Add(s.cacheTTL(ctx))}
	s.cmu.Unlock()
	return v, nil
}

// dialSSH opens host's path to Sentinel: flags + token from Coolify, the
// host's SSH login, then Sentinel's container IP (docker inspect).
func (s *SentinelService) dialSSH(ctx context.Context, host *models.Host) (*sentinelConn, error) {
	cc, err := CoolifyClient(ctx, s.db)
	if err != nil {
		return nil, unavailable(SentinelReasonCoolifyOff, err.Error())
	}
	uuid := *host.CoolifyServerUUID
	srv, err := cc.GetServer(uuid)
	if err != nil {
		return nil, unavailable(SentinelReasonError, "coolify: "+err.Error())
	}
	st, err := cc.GetSentinel(uuid)
	if err != nil {
		return nil, unavailable(SentinelReasonError, "coolify: "+err.Error())
	}
	if st.Token == "" {
		return nil, unavailable(SentinelReasonNoToken, "")
	}

	if host.User == "" {
		return nil, unavailable(SentinelReasonSSHConfig, "host has no user configured")
	}
	_, auth, err := ResolveSSHAuth(ctx, s.db, host, "")
	var cfgErr *SSHConfigError
	if errors.As(err, &cfgErr) {
		return nil, unavailable(SentinelReasonSSHConfig, cfgErr.Msg)
	}
	if err != nil {
		return nil, unavailable(SentinelReasonError, err.Error())
	}
	client, err := sshtest.Dial(host.Hostname, host.Port, host.User, auth)
	if err != nil {
		return nil, unavailable(SentinelReasonSSHFailed, err.Error())
	}

	ip, err := sentinelIP(client)
	if err != nil {
		client.Close()
		return nil, err
	}
	api := sentinel.NewClient(ip+":"+sentinelPort, st.Token, client.DialContext)
	if _, err := api.Version(ctx); err != nil {
		client.Close()
		if strings.Contains(err.Error(), "prohibited") {
			return nil, unavailable(SentinelReasonNoForward, "sshd AllowTcpForwarding is off")
		}
		if errors.Is(err, sentinel.ErrUnauthorized) {
			return nil, unavailable(SentinelReasonNoToken, "Sentinel rejected the token")
		}
		return nil, unavailable(SentinelReasonError, err.Error())
	}
	return &sentinelConn{ssh: client, api: api,
		metrics: srv.Settings.IsMetricsEnabled, traffic: srv.Settings.IsTrafficAnalyticsEnabled}, nil
}

// sentinelIP reads coolify-sentinel's container address, falling back to
// passwordless sudo when the SSH user isn't in the docker group.
// ponytail: no `docker exec curl` transport for hosts with TCP forwarding
// off; add one when such a host shows up (reason no_tcp_forwarding).
func sentinelIP(client *ssh.Client) (string, error) {
	cmd := "docker inspect -f '" + sentinelInspectIPTemplate + "' " + sentinelContainer
	out, err := runSSH(client, cmd)
	if err != nil && strings.Contains(strings.ToLower(out), "permission denied") {
		out, err = runSSH(client, "sudo -n "+cmd)
	}
	low := strings.ToLower(out)
	switch {
	case strings.Contains(low, "no such object") || strings.Contains(low, "no such container"):
		return "", unavailable(SentinelReasonNoSentinel, "")
	case err != nil && (strings.Contains(low, "permission denied") || strings.Contains(low, "password is required")):
		return "", unavailable(SentinelReasonNoDocker, "")
	case err != nil:
		return "", unavailable(SentinelReasonError, strings.TrimSpace(out))
	}
	for _, f := range strings.Fields(out) {
		if f != "" && f != "<no value>" {
			return f, nil
		}
	}
	return "", unavailable(SentinelReasonNoSentinel, "container has no IP (not running?)")
}

func runSSH(client *ssh.Client, cmd string) (string, error) {
	sess, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()
	t := time.AfterFunc(15*time.Second, func() { sess.Close() })
	defer t.Stop()
	// CombinedOutput, not one buffer as Stdout and Stderr: the two streams are
	// copied by separate goroutines, and an unguarded buffer gets corrupted.
	out, err := sess.CombinedOutput(cmd)
	return string(out), err
}

// conn returns host's pooled connection, opening one when needed (dialed
// reports that); closes connections idle past sentinelIdleTimeout on the way.
// fresh drops the pooled one first.
func (s *SentinelService) conn(ctx context.Context, host *models.Host, fresh bool) (c *sentinelConn, dialed bool, err error) {
	if !s.Enabled(ctx) {
		return nil, false, unavailable(SentinelReasonDisabled, "")
	}
	if host.CoolifyServerUUID == nil || *host.CoolifyServerUUID == "" {
		return nil, false, unavailable(SentinelReasonNotLinked, "")
	}
	now := time.Now()
	s.mu.Lock()
	for id, sl := range s.slots {
		if id != host.ID && sl.mu.TryLock() {
			if sl.conn != nil && now.Sub(sl.conn.lastUsed) > sentinelIdleTimeout {
				sl.conn.close()
				sl.conn = nil
			}
			sl.mu.Unlock()
		}
	}
	sl, ok := s.slots[host.ID]
	if !ok {
		sl = &sentinelSlot{}
		s.slots[host.ID] = sl
	}
	s.mu.Unlock()

	sl.mu.Lock()
	defer sl.mu.Unlock()
	if sl.conn != nil && (fresh || now.Sub(sl.conn.lastUsed) > sentinelIdleTimeout) {
		sl.conn.close()
		sl.conn = nil
	}
	if sl.conn == nil {
		c, err := s.dial(ctx, host)
		if err != nil {
			return nil, true, err
		}
		sl.conn, dialed = c, true
	}
	sl.conn.lastUsed = now
	return sl.conn, dialed, nil
}

// with runs fn on host's connection; a pooled connection that fails is
// replaced once (Sentinel recreated → new IP; token rotated; SSH dropped).
func (s *SentinelService) with(ctx context.Context, host *models.Host, fn func(*sentinelConn) error) error {
	c, dialed, err := s.conn(ctx, host, false)
	if err != nil {
		return err
	}
	err = fn(c)
	var un *SentinelUnavailable
	if err == nil || errors.As(err, &un) || dialed {
		return err
	}
	if c, _, err = s.conn(ctx, host, true); err != nil {
		return err
	}
	return fn(c)
}

// Close drops every pooled connection (shutdown).
func (s *SentinelService) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sl := range s.slots {
		sl.mu.Lock()
		sl.conn.close()
		sl.conn = nil
		sl.mu.Unlock()
	}
}

// --- live reads ----------------------------------------------------------

// SentinelStatus is what the host card shows.
type SentinelStatus struct {
	Available bool                 `json:"available"`
	Reason    string               `json:"reason,omitempty"`
	Detail    string               `json:"detail,omitempty"`
	Metrics   bool                 `json:"metrics"`
	Traffic   bool                 `json:"traffic"`
	Version   string               `json:"version,omitempty"`
	Push      *sentinel.PushStatus `json:"push,omitempty"`
}

func statusOf(err error) SentinelStatus {
	var un *SentinelUnavailable
	if errors.As(err, &un) {
		return SentinelStatus{Reason: un.Reason, Detail: un.Detail}
	}
	return SentinelStatus{Reason: SentinelReasonError, Detail: err.Error()}
}

// Status checks host's Sentinel end to end (never cached: it's the probe).
func (s *SentinelService) Status(ctx context.Context, host *models.Host) SentinelStatus {
	var st SentinelStatus
	err := s.with(ctx, host, func(c *sentinelConn) error {
		v, err := c.api.Version(ctx)
		if err != nil {
			return err
		}
		ps, err := c.api.PushStatus(ctx)
		if err != nil {
			return err
		}
		st = SentinelStatus{Available: true, Metrics: c.metrics, Traffic: c.traffic, Version: v, Push: ps}
		return nil
	})
	if err != nil {
		return statusOf(err)
	}
	return st
}

// metricsRanges are the host/container history windows (Sentinel keeps 7d).
var metricsRanges = map[string]time.Duration{"1h": time.Hour, "24h": 24 * time.Hour, "7d": 7 * 24 * time.Hour}

// MetricsRange validates a metrics window, defaulting to 24h.
func MetricsRange(r string) (string, time.Duration, error) {
	if r == "" {
		r = "24h"
	}
	d, ok := metricsRanges[r]
	if !ok {
		return "", 0, fmt.Errorf("range must be 1h, 24h or 7d")
	}
	return r, d, nil
}

// TrafficRange validates a traffic window, defaulting to 24h.
func TrafficRange(r string) (string, time.Duration, error) {
	if r == "" {
		r = "24h"
	}
	d, ok := sentinel.Ranges[r]
	if !ok {
		return "", 0, fmt.Errorf("range must be 24h, 7d or 30d")
	}
	return r, d, nil
}

const chartPoints = 500

// HostMetrics is a host's history plus its latest sample.
type HostMetrics struct {
	Range   string           `json:"range"`
	CPU     []sentinel.Point `json:"cpu"`
	Memory  []sentinel.Point `json:"memory"`
	Load    []sentinel.Point `json:"load"`
	Current struct {
		CPU      float64 `json:"cpu"`
		Memory   float64 `json:"memory"`
		Disk     float64 `json:"disk"`
		Load1    float64 `json:"load1"`
		UptimeS  float64 `json:"uptime_s"`
		SampleAt int64   `json:"sample_at"`
	} `json:"current"`
}

// HostMetrics reads host CPU/memory/load history for rng (1h|24h|7d).
func (s *SentinelService) HostMetrics(ctx context.Context, host *models.Host, rng string) (*HostMetrics, error) {
	rng, d, err := MetricsRange(rng)
	if err != nil {
		return nil, err
	}
	return cached(ctx, s, fmt.Sprintf("metrics:%d:%s", host.ID, rng), func() (*HostMetrics, error) {
		var m HostMetrics
		err := s.with(ctx, host, func(c *sentinelConn) error {
			if !c.metrics {
				return unavailable(SentinelReasonMetricsOff, "")
			}
			from := time.Now().Add(-d)
			var err error
			if m.CPU, err = c.api.CPUHistory(ctx, from); err != nil {
				return err
			}
			if m.Memory, err = c.api.MemoryHistory(ctx, from); err != nil {
				return err
			}
			if m.Load, err = c.api.LoadHistory(ctx, from); err != nil {
				return err
			}
			sum, err := c.api.Summary(ctx)
			if err != nil {
				return err
			}
			if sum.CPU != nil {
				m.Current.CPU, m.Current.SampleAt = float64(sum.CPU.Percent), int64(sum.CPU.Time)
			}
			if sum.Memory != nil {
				m.Current.Memory = float64(sum.Memory.UsedPercent)
			}
			if sum.Load != nil {
				m.Current.Load1 = float64(sum.Load.Load1)
			}
			if sum.Host != nil {
				m.Current.UptimeS = float64(sum.Host.UptimeSeconds)
			}
			m.Current.Disk = sum.DiskPct()
			return nil
		})
		m.Range = rng
		m.CPU = sentinel.Downsample(m.CPU, chartPoints)
		m.Memory = sentinel.Downsample(m.Memory, chartPoints)
		m.Load = sentinel.Downsample(m.Load, chartPoints)
		return &m, err
	})
}

// ContainerView is one live container with the Bridge service it maps to.
type ContainerView struct {
	Name         string   `json:"name"`
	State        string   `json:"state"`
	Health       string   `json:"health"`
	RestartCount int      `json:"restart_count"`
	CPUPct       *float64 `json:"cpu_pct"` // of one core
	MemPct       *float64 `json:"mem_pct"`
	MemUsed      *float64 `json:"mem_used_bytes"`
	ServiceID    *int64   `json:"service_id,omitempty"`
	ServiceName  string   `json:"service_name,omitempty"`
}

// Containers lists host's live containers, each linked to its service.
func (s *SentinelService) Containers(ctx context.Context, host *models.Host) ([]ContainerView, error) {
	return cached(ctx, s, fmt.Sprintf("containers:%d", host.ID), func() ([]ContainerView, error) {
		var cs []sentinel.Container
		err := s.with(ctx, host, func(c *sentinelConn) error {
			if !c.metrics {
				return unavailable(SentinelReasonMetricsOff, "")
			}
			var err error
			cs, err = c.api.ContainersCurrent(ctx)
			return err
		})
		if err != nil {
			return nil, err
		}
		svcs, err := store.NewServiceRepo(s.db.SQL).ListDiscoveredByHost(ctx, host.ID)
		if err != nil {
			return nil, err
		}
		owner := map[string]*models.Service{}
		for i := range svcs {
			if ct := containerFor(cs, svcs[i]); ct != nil {
				owner[ct.ID] = &svcs[i]
			}
		}
		out := make([]ContainerView, 0, len(cs))
		for _, ct := range cs {
			v := ContainerView{Name: ct.ID, State: ct.Status.State, Health: ct.Status.Health, RestartCount: ct.Status.RestartCount}
			if ct.CPU != nil {
				p := float64(ct.CPU.Percent)
				v.CPUPct = &p
			}
			if ct.Memory != nil {
				p, u := float64(ct.Memory.UsedPercent), float64(ct.Memory.Used)
				v.MemPct, v.MemUsed = &p, &u
			}
			if svc := owner[ct.ID]; svc != nil {
				id := svc.ID
				v.ServiceID, v.ServiceName = &id, svc.Nickname
			}
			out = append(out, v)
		}
		return out, nil
	})
}

// serviceHost resolves the Coolify-linked host a service runs on.
func (s *SentinelService) serviceHost(ctx context.Context, svc *models.Service) (*models.Host, error) {
	h, err := s.repo.CoolifyHostForService(ctx, svc.ID)
	if err != nil {
		return nil, err
	}
	if h == nil {
		return nil, unavailable(SentinelReasonNotLinked, "")
	}
	return h, nil
}

// appKeys is the host's Sentinel app key list, cached like any read.
func (s *SentinelService) appKeys(ctx context.Context, host *models.Host, c *sentinelConn) ([]string, error) {
	return cached(ctx, s, fmt.Sprintf("apps:%d", host.ID), func() ([]string, error) { return c.api.TrafficApps(ctx) })
}

// ServiceTraffic is a service's traffic over a window.
type ServiceTraffic struct {
	Range    string            `json:"range"`
	Keys     []string          `json:"keys"` // Sentinel app keys matched; empty = no traffic through the proxy
	Overview sentinel.Overview `json:"overview"`
	Series   []sentinel.Bucket `json:"series"`
	Paths    []sentinel.Path   `json:"paths"`
}

// ServiceTraffic reads a service's traffic (24h|7d|30d) across its app keys.
func (s *SentinelService) ServiceTraffic(ctx context.Context, svc *models.Service, rng string) (*ServiceTraffic, error) {
	rng, d, err := TrafficRange(rng)
	if err != nil {
		return nil, err
	}
	host, err := s.serviceHost(ctx, svc)
	if err != nil {
		return nil, err
	}
	return cached(ctx, s, fmt.Sprintf("traffic:%d:%s", svc.ID, rng), func() (*ServiceTraffic, error) {
		t := &ServiceTraffic{Range: rng, Keys: []string{}}
		err := s.with(ctx, host, func(c *sentinelConn) error {
			if !c.traffic {
				return unavailable(SentinelReasonTrafficOff, "")
			}
			all, err := s.appKeys(ctx, host, c)
			if err != nil {
				return err
			}
			t.Keys = trafficKeys(*svc, all)
			to := time.Now()
			from := to.Add(-d)
			var os []*sentinel.Overview
			var ss [][]sentinel.Bucket
			var ps [][]sentinel.Path
			for _, k := range t.Keys {
				o, err := c.api.Overview(ctx, sentinel.ScopeApp, k, from, to)
				if err != nil {
					return err
				}
				b, err := c.api.Series(ctx, sentinel.ScopeApp, k, rng)
				if err != nil {
					return err
				}
				p, err := c.api.Paths(ctx, sentinel.ScopeApp, k, from, to, 20)
				if err != nil {
					return err
				}
				os, ss, ps = append(os, o), append(ss, b), append(ps, p)
			}
			t.Overview = sumOverview(os)
			t.Series = mergeSeries(ss)
			t.Paths = topPaths(ps, 20)
			return nil
		})
		if t.Series == nil {
			t.Series = []sentinel.Bucket{}
		}
		if t.Paths == nil {
			t.Paths = []sentinel.Path{}
		}
		return t, err
	})
}

// ServiceBreakdown is one dimension of a service's traffic.
func (s *SentinelService) ServiceBreakdown(ctx context.Context, svc *models.Service, dim, rng string) ([]sentinel.BreakdownRow, error) {
	if !sentinel.Dimensions[dim] {
		return nil, fmt.Errorf("unknown dimension %q", dim)
	}
	rng, d, err := TrafficRange(rng)
	if err != nil {
		return nil, err
	}
	host, err := s.serviceHost(ctx, svc)
	if err != nil {
		return nil, err
	}
	return cached(ctx, s, fmt.Sprintf("breakdown:%d:%s:%s", svc.ID, dim, rng), func() ([]sentinel.BreakdownRow, error) {
		var rows [][]sentinel.BreakdownRow
		err := s.with(ctx, host, func(c *sentinelConn) error {
			if !c.traffic {
				return unavailable(SentinelReasonTrafficOff, "")
			}
			all, err := s.appKeys(ctx, host, c)
			if err != nil {
				return err
			}
			to := time.Now()
			for _, k := range trafficKeys(*svc, all) {
				r, err := c.api.Breakdown(ctx, sentinel.ScopeApp, k, dim, to.Add(-d), to, 20)
				if err != nil {
					return err
				}
				rows = append(rows, r)
			}
			return nil
		})
		return mergeBreakdown(rows, 20), err
	})
}

// ServiceResources is a service container's CPU/memory history.
type ServiceResources struct {
	Range     string           `json:"range"`
	Container string           `json:"container"` // "" = no live container matched
	CPU       []sentinel.Point `json:"cpu"`
	Memory    []sentinel.Point `json:"memory"`
}

// ServiceResources reads the service container's history (1h|24h|7d).
func (s *SentinelService) ServiceResources(ctx context.Context, svc *models.Service, rng string) (*ServiceResources, error) {
	rng, d, err := MetricsRange(rng)
	if err != nil {
		return nil, err
	}
	host, err := s.serviceHost(ctx, svc)
	if err != nil {
		return nil, err
	}
	return cached(ctx, s, fmt.Sprintf("resources:%d:%s", svc.ID, rng), func() (*ServiceResources, error) {
		r := &ServiceResources{Range: rng, CPU: []sentinel.Point{}, Memory: []sentinel.Point{}}
		err := s.with(ctx, host, func(c *sentinelConn) error {
			if !c.metrics {
				return unavailable(SentinelReasonMetricsOff, "")
			}
			cs, err := c.api.ContainersCurrent(ctx)
			if err != nil {
				return err
			}
			ct := containerFor(cs, *svc)
			if ct == nil {
				return nil
			}
			r.Container = ct.ID
			from := time.Now().Add(-d)
			if r.CPU, err = c.api.ContainerCPU(ctx, ct.ID, from); err != nil {
				return err
			}
			r.Memory, err = c.api.ContainerMemory(ctx, ct.ID, from)
			return err
		})
		r.CPU = sentinel.Downsample(r.CPU, chartPoints)
		r.Memory = sentinel.Downsample(r.Memory, chartPoints)
		return r, err
	})
}

// --- sync ----------------------------------------------------------------

// SentinelSyncHost is one host's line in the sync report.
type SentinelSyncHost struct {
	HostID   int64  `json:"host_id"`
	Slug     string `json:"slug"`
	Nickname string `json:"nickname"`
	State    string `json:"state"`
	Detail   string `json:"detail,omitempty"`
	Services int    `json:"services"` // service snapshots written
}

// SentinelSyncReport is the outcome of a sync, host by host.
type SentinelSyncReport struct {
	Hosts []SentinelSyncHost `json:"hosts"`
	OK    int                `json:"ok"`
}

// Sync snapshots every Coolify-linked host, sentinelSyncParallelism at a
// time. One host failing never fails the sync: its state records why.
func (s *SentinelService) Sync(ctx context.Context) (*SentinelSyncReport, error) {
	if !s.Enabled(ctx) {
		return nil, unavailable(SentinelReasonDisabled, "")
	}
	hosts, err := s.repo.CoolifyHosts(ctx)
	if err != nil {
		return nil, err
	}
	rep := &SentinelSyncReport{Hosts: make([]SentinelSyncHost, len(hosts))}
	sem := make(chan struct{}, sentinelSyncParallelism)
	var wg sync.WaitGroup
	for i := range hosts {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			hctx, cancel := context.WithTimeout(ctx, sentinelHostSyncTimeout)
			defer cancel()
			rep.Hosts[i] = s.syncHost(hctx, &hosts[i])
		}(i)
	}
	wg.Wait()
	for _, h := range rep.Hosts {
		if h.State == SentinelOK {
			rep.OK++
		}
	}
	return rep, nil
}

func f64(v float64) *float64 { return &v }
func i64(v int64) *int64     { return &v }

// syncHost writes host's snapshot and its services' snapshots.
func (s *SentinelService) syncHost(ctx context.Context, host *models.Host) SentinelSyncHost {
	res := SentinelSyncHost{HostID: host.ID, Slug: host.OficialSlug, Nickname: host.Nickname, State: SentinelOK}
	snap := &models.SentinelHostSnapshot{HostID: host.ID, State: SentinelOK}
	var svcSnaps []models.SentinelServiceSnapshot

	err := s.with(ctx, host, func(c *sentinelConn) error {
		snap.MetricsEnabled, snap.TrafficEnabled = c.metrics, c.traffic
		svcSnaps = nil
		if ps, err := c.api.PushStatus(ctx); err == nil {
			ok := ps.ConsecutiveFailures == 0 && ps.LastSuccessAt != nil
			snap.PushOK, snap.LastPushAt = &ok, ps.LastSuccessAt
		}

		var cs []sentinel.Container
		if c.metrics {
			from := time.Now().Add(-24 * time.Hour)
			cpu, err := c.api.CPUHistory(ctx, from)
			if err != nil {
				return err
			}
			mem, err := c.api.MemoryHistory(ctx, from)
			if err != nil {
				return err
			}
			if len(cpu) > 0 {
				snap.CPUAvg24h, snap.CPUP95_24h = f64(sentinel.Mean(cpu)), f64(sentinel.P95(cpu))
			}
			if len(mem) > 0 {
				snap.MemP95_24h = f64(sentinel.P95(mem))
			}
			sum, err := c.api.Summary(ctx)
			if err != nil {
				return err
			}
			if len(sum.Disk) > 0 {
				snap.DiskPct = f64(sum.DiskPct())
			}
			if sum.Load != nil {
				snap.Load1 = f64(float64(sum.Load.Load1))
			}
			if cs, err = c.api.ContainersCurrent(ctx); err != nil {
				return err
			}
		}

		var keys []string
		if c.traffic {
			var err error
			if keys, err = c.api.TrafficApps(ctx); err != nil {
				return err
			}
		}

		svcs, err := store.NewServiceRepo(s.db.SQL).ListDiscoveredByHost(ctx, host.ID)
		if err != nil {
			return err
		}
		to := time.Now()
		for _, svc := range svcs {
			v := models.SentinelServiceSnapshot{ServiceID: svc.ID, HostID: host.ID}
			matched := false
			if ct := containerFor(cs, svc); ct != nil {
				matched = true
				v.ContainerState, v.ContainerHealth = ct.Status.State, ct.Status.Health
				rc := ct.Status.RestartCount
				v.RestartCount = &rc
				if ct.CPU != nil {
					v.CPUPct = f64(float64(ct.CPU.Percent))
				}
				if ct.Memory != nil {
					v.MemPct = f64(float64(ct.Memory.UsedPercent))
				}
			}
			if ks := trafficKeys(svc, keys); len(ks) > 0 {
				matched = true
				var os []*sentinel.Overview
				for _, k := range ks {
					o, err := c.api.Overview(ctx, sentinel.ScopeApp, k, to.Add(-24*time.Hour), to)
					if err != nil {
						return err
					}
					os = append(os, o)
				}
				o := sumOverview(os)
				v.Req24h, v.S4xx24h, v.S5xx24h = i64(o.Requests), i64(o.Status.S4xx), i64(o.Status.S5xx)
				v.Visitors24h = i64(o.UniqueVisitors)
				if o.Requests > 0 {
					v.P50Ms, v.P95Ms, v.P99Ms = f64(o.Latency.P50), f64(o.Latency.P95), f64(o.Latency.P99)
				}
				v.TraefikBypass = publishesPort(svc.ContainerPorts)
			}
			if matched {
				svcSnaps = append(svcSnaps, v)
			}
		}
		return nil
	})

	if err != nil {
		st := statusOf(err)
		snap = &models.SentinelHostSnapshot{HostID: host.ID, State: st.Reason, Detail: st.Detail,
			MetricsEnabled: snap.MetricsEnabled, TrafficEnabled: snap.TrafficEnabled}
		svcSnaps = nil
		res.State, res.Detail = st.Reason, st.Detail
	}
	if err := s.repo.UpsertHost(ctx, snap); err != nil {
		log.Printf("[sentinel] host %s: save snapshot: %v", host.OficialSlug, err)
		res.State, res.Detail = SentinelReasonError, "save snapshot: "+err.Error()
		return res
	}
	keep := make([]int64, 0, len(svcSnaps))
	for i := range svcSnaps {
		if err := s.repo.UpsertService(ctx, &svcSnaps[i]); err != nil {
			log.Printf("[sentinel] host %s: save service %d: %v", host.OficialSlug, svcSnaps[i].ServiceID, err)
			continue
		}
		keep = append(keep, svcSnaps[i].ServiceID)
	}
	if err := s.repo.PruneServices(ctx, host.ID, keep); err != nil {
		log.Printf("[sentinel] host %s: prune services: %v", host.OficialSlug, err)
	}
	res.Services = len(keep)
	return res
}
