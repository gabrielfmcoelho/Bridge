package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// scansecrets.go — secrets passed on a command line the scan stores (a Redis
// password given to JBoss as -DcacheServerPassword=…) go to the vault as the
// host's scan secrets; the scan keeps only {{secret:<id>}}, or {{secret}} for
// a value the vault doesn't hold. No value is ever logged or put in an error.

// FoundSecret is one secret in a command line: the key it was passed under
// and the value's byte span [Start, End) in the command.
type FoundSecret struct {
	Key, Value string
	Start, End int
}

var (
	secretKeyRe = regexp.MustCompile(`(?i)(pass(word|wd)?|pwd|secret|token|api[-_]?key|credential)`)
	// -D<name>=<v>, --<name>=<v>, -<name>=<v> and NAME=<v>, each its own word
	// (a crontab line's assignment follows the schedule).
	assignRe = regexp.MustCompile(`(?:^|\s)(?:-D|--?)?([A-Za-z_][A-Za-z0-9_.\-]*)=(\S*)`)
	// --<name> <v>, the value not itself a flag.
	flagArgRe = regexp.MustCompile(`(?:^|\s)--([A-Za-z_][A-Za-z0-9_.\-]*)\s+([^\s-]\S*)`)
	// scheme://user:pass@host
	urlUserRe     = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.\-]*://([^\s:/@]*):([^\s@/]+)@([^\s/:?#@]+)`)
	wordRe        = regexp.MustCompile(`\S+`)
	envAssignRe   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
	interpreterRe = regexp.MustCompile(`^(python|node|nodejs|php|ruby)[0-9.]*$`)
	nonAlnumRe    = regexp.MustCompile(`[^a-z0-9]+`)
	underscoresRe = regexp.MustCompile(`_+`)
)

var mysqlClients = map[string]bool{"mysql": true, "mysqldump": true, "mysqladmin": true}

// FindSecrets returns the secrets in cmd, in order, spans never overlapping.
// Empty values, references ($VAR, ${VAR}) and redaction tokens are skipped.
func FindSecrets(cmd string) []FoundSecret {
	var found []FoundSecret
	add := func(key string, s, e int) {
		// A quoted value (a crontab line) is what's inside the quotes, spaces too.
		if s < e && (cmd[s] == '"' || cmd[s] == '\'') {
			if j := strings.IndexByte(cmd[s+1:], cmd[s]); j >= 0 {
				s, e = s+1, s+1+j
			}
		}
		v := cmd[s:e]
		if v == "" || v[0] == '$' || strings.HasPrefix(v, "{{secret") {
			return
		}
		found = append(found, FoundSecret{Key: key, Value: v, Start: s, End: e})
	}
	for _, re := range []*regexp.Regexp{assignRe, flagArgRe} {
		for _, m := range re.FindAllStringSubmatchIndex(cmd, -1) {
			if key := cmd[m[2]:m[3]]; secretKeyRe.MatchString(key) {
				add(key, m[4], m[5])
			}
		}
	}
	for _, m := range urlUserRe.FindAllStringSubmatchIndex(cmd, -1) {
		add("url-"+cmd[m[2]:m[3]]+"-"+cmd[m[6]:m[7]], m[4], m[5])
	}
	mysql := false
	for _, w := range wordRe.FindAllStringIndex(cmd, -1) {
		word := cmd[w[0]:w[1]]
		if mysqlClients[path.Base(strings.Trim(word, `"'`))] {
			mysql = true
		} else if mysql && strings.HasPrefix(word, "-p") {
			add("mysql-password", w[0]+2, w[1])
		}
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].Start < found[j].Start })
	out, end := found[:0], 0
	for _, f := range found {
		switch {
		case f.Start >= end:
			out, end = append(out, f), f.End
		case f.End > end: // starts inside the previous value and ends past it: one span covers both
			p := &out[len(out)-1]
			p.End, p.Value, end = f.End, cmd[p.Start:f.End], f.End
		}
	}
	return out
}

// EnvVarName turns a key as spelled on the command line into a vault env_var
// name: camelCase → snake, anything else → "_", upper-cased
// (cacheServerPassword → CACHE_SERVER_PASSWORD).
func EnvVarName(key string) string {
	var b strings.Builder
	for i := 0; i < len(key); i++ {
		switch c := key[i]; {
		case c >= 'A' && c <= 'Z':
			if p := key[max(i-1, 0)]; i > 0 && (p >= 'a' && p <= 'z' || p >= '0' && p <= '9') {
				b.WriteByte('_')
			}
			b.WriteByte(c)
		case c >= 'a' && c <= 'z':
			b.WriteByte(c - 'a' + 'A')
		case c >= '0' && c <= '9':
			b.WriteByte(c)
		default:
			b.WriteByte('_')
		}
	}
	name := strings.Trim(underscoresRe.ReplaceAllString(b.String(), "_"), "_")
	if name == "" || name[0] >= '0' && name[0] <= '9' {
		name = "_" + name
	}
	return name
}

// SecretOrigin is the group a command line's secrets belong to, from the line
// alone, so a backfill of old scans and a fresh scan name them alike.
func SecretOrigin(cmd string) string {
	group, _, _ := commandOrigin(cmd)
	return group
}

// commandOrigin returns the group (scan-<origin>), where the secret was found
// (for its description) and the program. Java → the app home, else the jar
// of -jar; an interpreter → its script; anything else → argv[0]'s basename.
func commandOrigin(cmd string) (group, where, prog string) {
	args := strings.Fields(cmd)
	for len(args) > 1 && envAssignRe.MatchString(args[0]) {
		args = args[1:]
	}
	if len(args) == 0 {
		return vault.ScanSecretGroupPrefix + "unknown", "", ""
	}
	where, prog = args[0], path.Base(args[0])
	name := prog
	if home := sshtest.AppHome(cmd); prog == "java" && home != "" {
		where, name = home, home
	} else if prog == "java" || interpreterRe.MatchString(prog) {
		if s := scriptArg(args[1:], prog == "java"); s != "" {
			where, name = s, path.Base(s)
		}
	}
	return scanGroup(name), where, prog
}

// scriptArg is what the interpreter runs: -jar's or -m's argument, else (not
// java) the first argument that isn't a flag.
func scriptArg(args []string, java bool) string {
	for i, a := range args {
		switch {
		case (a == "-jar" || a == "-m") && i+1 < len(args):
			return args[i+1]
		case java || strings.HasPrefix(a, "-"):
		default:
			return a
		}
	}
	return ""
}

func scanGroup(origin string) string {
	if k := strings.Trim(nonAlnumRe.ReplaceAllString(strings.ToLower(origin), "-"), "-"); k != "" {
		return vault.ScanSecretGroupPrefix + k
	}
	return vault.ScanSecretGroupPrefix + "unknown"
}

// redactSpans replaces each found value (sorted, not overlapping, as
// FindSecrets returns them) with its token; the rest of cmd is untouched.
func redactSpans(cmd string, found []FoundSecret, token func(FoundSecret) string) string {
	var b strings.Builder
	last := 0
	for _, f := range found {
		b.WriteString(cmd[last:f.Start])
		b.WriteString(token(f))
		last = f.End
	}
	b.WriteString(cmd[last:])
	return b.String()
}

// cmdSource is a command line whose secrets are registered, with its origin.
type cmdSource struct {
	cmd                *string
	group, where, prog string
}

// scanCommandLines lists every command line the scan stores: processes and
// cron jobs, whose secrets are registered, and the lines only redacted —
// resource_top's (cut at 200 characters) and the legacy crontab lines.
func scanCommandLines(info *sshtest.VMInfo) (sources []cmdSource, redactOnly []*string) {
	for i := range info.ProcessDetails {
		c := &info.ProcessDetails[i].Command
		group, where, prog := commandOrigin(*c)
		sources = append(sources, cmdSource{c, group, where, prog})
	}
	if info.Cron != nil {
		for i := range info.Cron.Jobs {
			j := &info.Cron.Jobs[i]
			_, _, prog := commandOrigin(j.Command)
			sources = append(sources, cmdSource{&j.Command, scanGroup("cron-" + j.User), strings.TrimSpace("cron " + j.User), prog})
		}
	}
	if rt := info.ResourceTop; rt != nil {
		for _, list := range [][]sshtest.ResourceProcess{rt.TopCPU, rt.TopMem} {
			for i := range list {
				redactOnly = append(redactOnly, &list[i].Command)
			}
		}
	}
	for i := range info.CronJobs {
		redactOnly = append(redactOnly, &info.CronJobs[i])
	}
	return sources, redactOnly
}

// RecordScanSecrets stores every secret on info's processes and cron jobs as
// the host's scan secret, written by actorUserID (≤ 0: the system actor), and
// replaces each value in info by its token. Values sharing a (group, name)
// are named NAME, NAME_2, … in value order, so a re-scan listing them in
// another order names them alike. Every value is redacted whatever happens:
// one the vault refused or holds in the trash becomes {{secret}}. Returns
// how many secrets the vault holds for this scan.
func RecordScanSecrets(ctx context.Context, db *database.DB, hostID, actorUserID int64, info *sshtest.VMInfo) (int, error) {
	sources, redactOnly := scanCommandLines(info)
	type ident struct{ group, name string }
	values := map[ident]map[string]string{} // value → description
	found := make([][]FoundSecret, len(sources))
	for i, s := range sources {
		found[i] = FindSecrets(*s.cmd)
		for _, f := range found[i] {
			id := ident{s.group, EnvVarName(f.Key)}
			if values[id] == nil {
				values[id] = map[string]string{}
			}
			desc := spelling(*s.cmd, f) + " · " + s.prog + " · " + s.where
			if old, ok := values[id][f.Value]; !ok || desc < old {
				values[id][f.Value] = desc
			}
		}
	}

	const blank = "{{secret}}"
	tokens := map[string]string{}  // group, name, value → token
	byValue := map[string]string{} // value → token, for the redact-only lines
	var errs []error
	n := 0
	for id, vals := range values {
		for k, v := range slices.Sorted(maps.Keys(vals)) {
			name := id.name
			if k > 0 {
				name = fmt.Sprintf("%s_%d", id.name, k+1)
			}
			tok := blank
			switch sid, err := vault.UpsertScanSecret(ctx, db, hostID, actorUserID, id.group, name, v, vals[v]); {
			case err == nil:
				tok, n = fmt.Sprintf("{{secret:%d}}", sid), n+1
			case !errors.Is(err, vault.ErrScanSecretTrashed):
				errs = append(errs, fmt.Errorf("%s/%s: %w", id.group, name, err))
			}
			tokens[id.group+"\x00"+id.name+"\x00"+v] = tok
			if byValue[v] == "" || byValue[v] == blank {
				byValue[v] = tok
			}
		}
	}
	lookup := func(m map[string]string, key string) string {
		if t, ok := m[key]; ok {
			return t
		}
		return blank
	}
	for i, s := range sources {
		*s.cmd = redactSpans(*s.cmd, found[i], func(f FoundSecret) string {
			return lookup(tokens, s.group+"\x00"+EnvVarName(f.Key)+"\x00"+f.Value)
		})
	}
	for _, c := range redactOnly {
		*c = redactSpans(*c, FindSecrets(*c), func(f FoundSecret) string { return lookup(byValue, f.Value) })
	}
	return n, errors.Join(errs...)
}

// spelling is the key as the command line wrote it (-DcacheServerPassword).
func spelling(cmd string, f FoundSecret) string {
	pre := strings.Fields(strings.TrimRight(cmd[:f.Start], "=:'\" \t"))
	if len(pre) == 0 {
		return f.Key
	}
	return pre[len(pre)-1]
}

const scanSecretsBackfillFlag = "scan_secrets_backfill_v1"

// scanBackfillBatch is how many host_scans rows the backfill reads at a time.
var scanBackfillBatch = 200

// BackfillScanSecrets registers the secrets of every stored scan, in id order
// (a later scan's value wins), and rewrites each row that held one, redacted.
// Re-runnable: a row already redacted holds no value and is left alone.
// rows is the scans rewritten, secrets the secrets found in them.
func BackfillScanSecrets(ctx context.Context, db *database.DB) (rows, secrets int, err error) {
	ctx = store.WithSystemScope(ctx)
	repo := store.NewHostScanRepo(db.SQL)
	var firstErr error
	failed := 0
	for after := int64(0); ; {
		batch, err := repo.ListAfter(ctx, after, scanBackfillBatch)
		if err != nil {
			return rows, secrets, err
		}
		if len(batch) == 0 {
			break
		}
		for _, s := range batch {
			after = s.ID
			data, n, recErr := redactScanJSON(ctx, db, s.HostID, s.Data)
			secrets += n
			if recErr != nil {
				if failed++; firstErr == nil {
					firstErr = recErr
				}
			}
			if data == "" {
				continue
			}
			if err := repo.UpdateData(ctx, s.ID, data); err != nil {
				return rows, secrets, err
			}
			rows++
		}
	}
	if failed > 0 {
		return rows, secrets, fmt.Errorf("%d scan(s) with a secret the vault didn't take (blanked), first: %w", failed, firstErr)
	}
	return rows, secrets, nil
}

// redactScanJSON registers one stored scan's secrets and returns its JSON
// redacted, or "" when nothing changed. Only the command strings change: the
// row is rewritten through a generic tree, so fields this Bridge no longer
// knows and large numbers survive.
func redactScanJSON(ctx context.Context, db *database.DB, hostID int64, data string) (string, int, error) {
	var info sshtest.VMInfo
	if err := json.Unmarshal([]byte(data), &info); err != nil {
		// A field of another shape is skipped and the rest decoded; not JSON at all, nothing to read.
		if te := (*json.UnmarshalTypeError)(nil); !errors.As(err, &te) {
			return "", 0, nil
		}
	}
	sources, redactOnly := scanCommandLines(&info)
	refs := redactOnly
	for _, s := range sources {
		refs = append(refs, s.cmd)
	}
	before := make([]string, len(refs))
	for i, r := range refs {
		before[i] = *r
	}
	n, recErr := RecordScanSecrets(ctx, db, hostID, 0, &info)
	repl := map[string]string{}
	for i, r := range refs {
		if *r != before[i] {
			repl[before[i]] = *r
		}
	}
	if len(repl) == 0 {
		return "", n, recErr
	}
	var tree any
	dec := json.NewDecoder(strings.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&tree); err != nil {
		return "", n, errors.Join(recErr, err)
	}
	out, err := json.Marshal(replaceStrings(tree, repl))
	if err != nil {
		return "", n, errors.Join(recErr, err)
	}
	return string(out), n, recErr
}

// replaceStrings swaps every string in a decoded JSON tree that equals a key of repl.
func replaceStrings(v any, repl map[string]string) any {
	switch t := v.(type) {
	case string:
		if r, ok := repl[t]; ok {
			return r
		}
	case []any:
		for i := range t {
			t[i] = replaceStrings(t[i], repl)
		}
	case map[string]any:
		for k := range t {
			t[k] = replaceStrings(t[k], repl)
		}
	}
	return v
}

// RunScanSecretsBackfill runs BackfillScanSecrets once (flag
// scan_secrets_backfill_v1, set only when every row went through). A failure
// is retried at the next start. Logs counts only.
func RunScanSecretsBackfill(ctx context.Context, db *database.DB) {
	settings := store.NewAppSettingsRepo(db.SQL)
	if settings.Value(ctx, scanSecretsBackfillFlag) != "" {
		return
	}
	rows, secrets, err := BackfillScanSecrets(ctx, db)
	if err != nil {
		log.Printf("[scan] secrets backfill incomplete, retried at the next start: %d scan(s) rewritten, %d secret(s) found: %v", rows, secrets, err)
		return
	}
	if err := settings.Set(ctx, scanSecretsBackfillFlag, "1"); err != nil {
		log.Printf("[scan] secrets backfill: set flag: %v", err)
	}
	if rows > 0 {
		log.Printf("[scan] secrets backfill: %d scan(s) rewritten, %d secret(s) found", rows, secrets)
	}
}
