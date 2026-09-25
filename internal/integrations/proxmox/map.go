package proxmox

import (
	"fmt"
	"net/netip"
	"sort"
	"strings"
)

// Resource is one row of /cluster/resources. ID is "node/pve1", "qemu/101", ...
type Resource struct {
	ID       string  `json:"id"`
	Type     string  `json:"type"`
	Node     string  `json:"node"`
	Name     string  `json:"name"`
	VMID     int     `json:"vmid"`
	Status   string  `json:"status"`
	MaxCPU   float64 `json:"maxcpu"`
	MaxMem   float64 `json:"maxmem"`
	MaxDisk  float64 `json:"maxdisk"`
	Template int     `json:"template"`
}

// IsGuest reports whether r is a non-template VM or container.
func (r Resource) IsGuest() bool {
	return (r.Type == "qemu" || r.Type == "lxc") && r.Template == 0
}

// AgentIface is one interface from the QEMU guest agent.
type AgentIface struct {
	Name        string      `json:"name"`
	IPAddresses []AgentAddr `json:"ip-addresses"`
}

// AgentAddr is one address of an AgentIface.
type AgentAddr struct {
	IP   string `json:"ip-address"`
	Type string `json:"ip-address-type"` // ipv4 | ipv6
}

// LXCIface is one interface from /nodes/{n}/lxc/{id}/interfaces.
type LXCIface struct {
	Name string `json:"name"`
	Inet string `json:"inet"` // "10.0.0.5/24"
}

// Machine is a Proxmox node or guest, ready to become a Bridge host.
type Machine struct {
	ProxmoxID      string // "node/pve1" | "qemu/101" | "lxc/102"
	Kind           string // node | qemu | lxc
	Name           string
	Node           string // node it runs on (itself, for a node)
	VMID           int
	IP             string
	Running        bool
	CPU, RAM, Disk string
}

// TipoMaquina maps the kind onto the tipo_maquina enum.
func (m Machine) TipoMaquina() string {
	switch m.Kind {
	case "node":
		return "Bare Metal"
	case "lxc":
		return "Container"
	}
	return "VM"
}

// Machines turns /cluster/resources into nodes (first, so guests can resolve
// their parent) followed by non-template guests, both sorted by id. IPs come
// from nodeIPs (by node name) and guestIPs (by resource id).
func Machines(res []Resource, nodeIPs, guestIPs map[string]string) []Machine {
	var nodes, guests []Machine
	for _, r := range res {
		m := Machine{
			ProxmoxID: r.ID, Kind: r.Type, Name: r.Name, Node: r.Node, VMID: r.VMID,
			CPU: vcpu(r.MaxCPU), RAM: bytesText(r.MaxMem), Disk: bytesText(r.MaxDisk),
		}
		switch {
		case r.Type == "node":
			m.Name, m.IP, m.Running = r.Node, nodeIPs[r.Node], r.Status == "online"
			nodes = append(nodes, m)
		case r.IsGuest():
			if m.Name == "" {
				m.Name = fmt.Sprintf("%s-%d", r.Type, r.VMID)
			}
			m.IP, m.Running = guestIPs[r.ID], r.Status == "running"
			guests = append(guests, m)
		}
	}
	byID := func(ms []Machine) {
		sort.Slice(ms, func(i, j int) bool { return ms[i].ProxmoxID < ms[j].ProxmoxID })
	}
	byID(nodes)
	byID(guests)
	return append(nodes, guests...)
}

// AgentIPv4 picks the first global-unicast IPv4 the guest agent reports.
func AgentIPv4(ifs []AgentIface) string {
	for _, i := range ifs {
		for _, a := range i.IPAddresses {
			if a.Type == "ipv4" && usable(a.IP) {
				return a.IP
			}
		}
	}
	return ""
}

// LXCIPv4 picks the first global-unicast IPv4 of a container.
func LXCIPv4(ifs []LXCIface) string {
	for _, i := range ifs {
		ip, _, _ := strings.Cut(i.Inet, "/")
		if usable(ip) {
			return ip
		}
	}
	return ""
}

func usable(s string) bool {
	a, err := netip.ParseAddr(s)
	return err == nil && a.Is4() && a.IsGlobalUnicast()
}

func vcpu(n float64) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf("%.0f vCPU", n)
}

// bytesText renders a byte count as whole GB (MB below 1 GiB), "" for zero.
func bytesText(b float64) string {
	switch {
	case b <= 0:
		return ""
	case b < 1<<30:
		return fmt.Sprintf("%.0f MB", b/(1<<20))
	}
	return fmt.Sprintf("%.0f GB", b/(1<<30))
}
