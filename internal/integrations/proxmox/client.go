package proxmox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Client is a read-only Proxmox VE API client authenticated by API token.
type Client struct {
	baseURL    string
	auth       string
	httpClient *http.Client
}

// NewClient builds a client for baseURL (e.g. https://pve.example:8006).
// skipVerify accepts Proxmox's default self-signed certificate.
func NewClient(baseURL, tokenID, secret string, skipVerify bool) *Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if skipVerify {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // admin opt-in, self-signed PVE
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		auth:       "PVEAPIToken=" + tokenID + "=" + secret,
		httpClient: &http.Client{Timeout: 15 * time.Second, Transport: tr},
	}
}

// get fetches /api2/json<path> and decodes its "data" envelope into out.
func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api2/json"+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.auth)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("proxmox api GET %s: %s %s", path, resp.Status, strings.TrimSpace(string(body)))
	}
	return json.Unmarshal(body, &struct {
		Data any `json:"data"`
	}{Data: out})
}

// Version returns the PVE version string — the connection test.
func (c *Client) Version(ctx context.Context) (string, error) {
	var v struct {
		Version string `json:"version"`
	}
	err := c.get(ctx, "/version", &v)
	return v.Version, err
}

// Resources lists every cluster resource (nodes, guests, storage, ...).
func (c *Client) Resources(ctx context.Context) ([]Resource, error) {
	var out []Resource
	return out, c.get(ctx, "/cluster/resources", &out)
}

// NodeIPs maps node name → IP from /cluster/status (standalone nodes included).
func (c *Client) NodeIPs(ctx context.Context) (map[string]string, error) {
	var st []struct {
		Type string `json:"type"`
		Name string `json:"name"`
		IP   string `json:"ip"`
	}
	if err := c.get(ctx, "/cluster/status", &st); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, s := range st {
		if s.Type == "node" && s.IP != "" {
			out[s.Name] = s.IP
		}
	}
	return out, nil
}

// GuestIP returns the guest's first usable IPv4, via the QEMU guest agent or
// the LXC interfaces endpoint. "" with an error when it can't be read (agent
// not installed, missing VM.Monitor / VM.GuestAgent.Audit privilege, ...).
func (c *Client) GuestIP(ctx context.Context, r Resource) (string, error) {
	switch r.Type {
	case "qemu":
		var out struct {
			Result []AgentIface `json:"result"`
		}
		if err := c.get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/agent/network-get-interfaces", r.Node, r.VMID), &out); err != nil {
			return "", err
		}
		return AgentIPv4(out.Result), nil
	case "lxc":
		var out []LXCIface
		if err := c.get(ctx, fmt.Sprintf("/nodes/%s/lxc/%d/interfaces", r.Node, r.VMID), &out); err != nil {
			return "", err
		}
		return LXCIPv4(out), nil
	}
	return "", nil
}

// guestIPConcurrency bounds the per-guest IP lookups one sync fires at PVE.
const guestIPConcurrency = 10

// Machines reads the whole cluster: resources, node IPs and (for running
// guests, best-effort) guest IPs. Only the resource listing is fatal.
func (c *Client) Machines(ctx context.Context) ([]Machine, error) {
	res, err := c.Resources(ctx)
	if err != nil {
		return nil, err
	}
	nodeIPs, err := c.NodeIPs(ctx)
	if err != nil {
		nodeIPs = map[string]string{} // nodes still sync, matched by proxmox_id only
	}
	var (
		guestIPs = map[string]string{}
		mu       sync.Mutex
		wg       sync.WaitGroup
		sem      = make(chan struct{}, guestIPConcurrency)
	)
	for _, r := range res {
		if !r.IsGuest() || r.Status != "running" {
			continue
		}
		sem <- struct{}{}
		wg.Go(func() {
			defer func() { <-sem }()
			if ip, err := c.GuestIP(ctx, r); err == nil && ip != "" {
				mu.Lock()
				guestIPs[r.ID] = ip
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	return Machines(res, nodeIPs, guestIPs), nil
}
