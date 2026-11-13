package sshtest

import (
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// scanPath: non-interactive SSH on CentOS gets /usr/local/bin:/usr/bin only.
const scanPath = "export PATH=\"$PATH:/usr/local/sbin:/usr/sbin:/sbin\""

// retrySessionOpen retries while the peer refuses the channel (sshd MaxSessions,
// default 10); any other error returns at once. wait gets the attempt just failed.
func retrySessionOpen(open func() (*ssh.Session, error), attempts int, wait func(attempt int)) (*ssh.Session, error) {
	var sess *ssh.Session
	var err error
	for a := 1; a <= attempts; a++ {
		sess, err = open()
		var oce *ssh.OpenChannelError
		if err == nil || !errors.As(err, &oce) || oce.Reason != ssh.Prohibited {
			return sess, err
		}
		if a < attempts {
			wait(a)
		}
	}
	return sess, err
}

// newSession opens a session, riding out sshd's per-connection session cap
// (20 attempts, about 30 s worst case, spent before the command's own deadline).
func newSession(client *ssh.Client) (*ssh.Session, error) {
	return retrySessionOpen(client.NewSession, 20, func(a int) {
		time.Sleep(time.Duration(a) * 150 * time.Millisecond)
	})
}

// loginsServicesPortsScript is session 3: logins, services, service details and
// listening ports. last -500 gives per-user grouping headroom for hosts where
// one busy account dominates.
func loginsServicesPortsScript(delim2 string) string {
	return strings.Join([]string{
		scanPath,
		`last -500 -w 2>/dev/null | head -500`,
		`echo '` + delim2 + `'`,
		`ps aux 2>/dev/null | grep -E '(nginx|apache|httpd|php-fpm|node|python|java|postgres|mysql|mariadb|redis|mongo|docker|containerd|caddy|haproxy|traefik|pm2|gunicorn|uvicorn|supervisord)' | grep -v grep | awk '{print $11}' | sort -u | head -15`,
		`echo '` + delim2 + `'`,
		`ps aux 2>/dev/null | grep -E '(nginx|apache|httpd|php-fpm|node|python|java|postgres|mysql|mariadb|redis|mongo|caddy|haproxy|traefik|pm2|gunicorn|uvicorn|supervisord)' | grep -v grep | awk '{printf "%s  CPU:%.1f%%  MEM:%.1f%%  RSS:%sMB\n", $11, $3, $4, int($6/1024)}' | head -15`,
		`echo '` + delim2 + `'`,
		`ss -tlnp 2>/dev/null | awk 'NR>1{split($4,a,":"); port=a[length(a)]; proc=$7; gsub(/.*users:\(\("/,"",proc); gsub(/".*/,"",proc); if(port+0>0) print port " " proc}' | sort -n -u | head -20`,
	}, "; ")
}

// cleanServiceBinaries drops the trailing colon of process titles
// ("nginx: master process" yields "nginx:").
func cleanServiceBinaries(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = strings.TrimSuffix(l, ":")
	}
	return out
}
