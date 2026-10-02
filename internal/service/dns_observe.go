package service

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// observeClient answers with the first response it gets: no redirects
// followed (a 301 is the observation), any certificate accepted (the cert
// probe judges it), no keep-alive (each probe is one request).
var observeClient = &http.Client{
	Timeout: 8 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
	Transport: &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS10},
		DisableKeepAlives: true,
	},
}

// probeObservation resolves the domain and reads the HTTP status on 80 and
// 443 (an explicit host:port is used for both). It never fails: what didn't
// answer stays empty / 0.
func probeObservation(ctx context.Context, domain string, now time.Time) models.DNSObservation {
	o := models.DNSObservation{ObservedAt: &now}
	host, _ := certTarget(domain)
	if net.ParseIP(host) == nil {
		o.ObsRecordType, o.ObsTarget = resolve(ctx, host)
	}
	target := strings.TrimSpace(domain)
	o.ObsHTTPStatus = httpStatus(ctx, "http://"+target+"/")
	o.ObsHTTPSStatus = httpStatus(ctx, "https://"+target+"/")
	o.ObsStatus = obsStatus(o.ObsHTTPStatus, o.ObsHTTPSStatus)
	return o
}

// resolve returns ("CNAME", target) when the name is an alias, else
// ("A", addresses). Both empty when it doesn't resolve.
func resolve(ctx context.Context, host string) (kind, target string) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if cname, err := net.DefaultResolver.LookupCNAME(ctx, host); err == nil {
		if c := strings.TrimSuffix(cname, "."); !strings.EqualFold(c, host) {
			return "CNAME", c
		}
	}
	addrs, err := net.DefaultResolver.LookupHost(ctx, host)
	if err != nil || len(addrs) == 0 {
		return "", ""
	}
	return "A", strings.Join(addrs, ", ")
}

func httpStatus(ctx context.Context, url string) int {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0
	}
	resp, err := observeClient.Do(req)
	if err != nil {
		return 0
	}
	resp.Body.Close()
	return resp.StatusCode
}

// obsStatus sums up the two ports the way the ETIPI inventory does: the best
// answer wins — any 2xx/3xx is online, else a 4xx is "no_content" (up but
// serving nothing), else a 5xx is "error", nothing at all is "offline".
func obsStatus(codes ...int) string {
	best := 0
	for _, c := range codes {
		if c > 0 && (best == 0 || c < best) {
			best = c
		}
	}
	switch {
	case best == 0:
		return "offline"
	case best < 400:
		return "online"
	case best < 500:
		return "no_content"
	default:
		return "error"
	}
}
