package sshtest

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"
)

// scanPath: non-interactive SSH on CentOS gets /usr/local/bin:/usr/bin only.
const scanPath = "export PATH=\"$PATH:/usr/local/sbin:/usr/sbin:/sbin\""

// retrySessionOpen retries while the peer refuses the channel (sshd MaxSessions,
// default 10): "administratively prohibited" up to OpenSSH 7.5, "connect
// failed" since. Any other error returns at once. wait gets the attempt just failed.
func retrySessionOpen(open func() (*ssh.Session, error), attempts int, wait func(attempt int)) (*ssh.Session, error) {
	var sess *ssh.Session
	var err error
	for a := 1; a <= attempts; a++ {
		sess, err = open()
		var oce *ssh.OpenChannelError
		if err == nil || !errors.As(err, &oce) || (oce.Reason != ssh.Prohibited && oce.Reason != ssh.ConnectionFailed) {
			return sess, err
		}
		if a < attempts {
			wait(a)
		}
	}
	return sess, err
}

// maxScanSessions keeps a capture under sshd's MaxSessions (default 10), with
// room for the sessions sshd has not finished closing.
const maxScanSessions = 8

// scanState is what a capture keeps per client: its session slots and the
// commands that never ran.
type scanState struct {
	slots    chan struct{}
	refused  atomic.Int32 // the session never opened
	timedOut atomic.Int32 // cut at its deadline
}

func newScanState() *scanState {
	return &scanState{slots: make(chan struct{}, maxScanSessions)}
}

// scanStates: *ssh.Client → *scanState, for the length of a capture. Looked up
// by client, like the budget, so callers keep their signature.
var scanStates sync.Map

// trackScan gives client a scanState until the returned func is called.
func trackScan(client *ssh.Client) func() {
	scanStates.Store(client, newScanState())
	return func() { scanStates.Delete(client) }
}

// stateOf is client's scanState: nil outside a capture.
func stateOf(client *ssh.Client) *scanState {
	v, _ := scanStates.Load(client)
	st, _ := v.(*scanState)
	return st
}

// acquire waits for a session slot and returns what gives it back (once).
// Outside a capture (nil) there is no cap.
func (st *scanState) acquire() func() {
	if st == nil {
		return func() {}
	}
	st.slots <- struct{}{}
	return sync.OnceFunc(func() { <-st.slots })
}

// incompleteWarning says how many commands never ran, or "" when all did:
// their probes came back empty, which would otherwise read as "nothing found".
func (st *scanState) incompleteWarning() string {
	if st == nil {
		return ""
	}
	refused, timedOut := st.refused.Load(), st.timedOut.Load()
	if refused+timedOut == 0 {
		return ""
	}
	return fmt.Sprintf("%d scan commands did not complete (%d refused by sshd, %d timed out). Parts of this scan may be missing.",
		refused+timedOut, refused, timedOut)
}

// scanSession is an ssh.Session that gives its slot back when closed.
type scanSession struct {
	*ssh.Session
	release func()
}

func (s *scanSession) Close() error {
	err := s.Session.Close()
	s.release()
	return err
}

// newSession opens a session. Inside a capture it first waits for one of the
// capture's slots, so sshd's per-connection cap is not reached; the retry
// (20 attempts, about 30 s worst case, spent before the command's own
// deadline) stays for the sessions sshd is still closing.
func newSession(client *ssh.Client) (*scanSession, error) {
	st := stateOf(client)
	release := st.acquire()
	sess, err := retrySessionOpen(client.NewSession, 20, func(a int) {
		time.Sleep(time.Duration(a) * 150 * time.Millisecond)
	})
	if err != nil {
		release()
		if st != nil {
			st.refused.Add(1)
		}
		return nil, err
	}
	return &scanSession{Session: sess, release: release}, nil
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
