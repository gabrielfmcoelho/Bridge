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
	"strconv"
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
	// A key starts a word: after a space, a quote (sh -c "…"), "(", ";", "&" or
	// "?" (a query string), or "|". The patterns stop at the key, the value is
	// read by hand, so a value never hides the next key.
	keyStart = `(?:^|[\s"'(;&?|])`
	// -D<name>=, --<name>=, -<name>= and NAME=.
	assignRe = regexp.MustCompile(keyStart + `(?:-D|--?)?([A-Za-z_][A-Za-z0-9_.\-]*)=`)
	// --<name> followed by whitespace and a value that is not itself a flag.
	flagArgRe = regexp.MustCompile(keyStart + `--([A-Za-z_][A-Za-z0-9_.\-]*)`)
	// scheme://user[:pass]@host — userinfo ends at the last "@", as Go's and
	// Python's URL parsers read it. A user already redacted is one token.
	urlUserRe = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.\-]*://(\{\{secret(?::\d+)?\}\}|[^\s:/@?#]*)(?::([^\s/]*))?@([^\s/:?#@]+)`)
	// A URL's user that is a credential, not a login (https://<token>@host):
	// a known token prefix, or 20+ characters of a token's alphabet.
	tokenUserRe = regexp.MustCompile(`^(gh[pousr]_|github_pat_|glpat-|gldt-|xox[abp]-)|^[A-Za-z0-9_\-]{20,}$`)
	// A resource_top line cut inside a URL's userinfo: no "@" is left. With no
	// ":" the cut part is a user or a host, blanked unless it reads as a host.
	cutURLRe  = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.\-]*://(?:([^\s:/@]*):)?([^\s@/]+)…$`)
	cutHostRe = regexp.MustCompile(`^[a-z0-9-]+(\.[a-z0-9-]*)+$`)
	// Not values: a reference ($NAME, ${NAME}) or a redaction token.
	notValueRe    = regexp.MustCompile(`^(\$(\{[A-Za-z_]\w*\}|[A-Za-z_]\w*)|\{\{secret(:\d+)?\}\})$`)
	wordRe        = regexp.MustCompile(`\S+`)
	envAssignRe   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
	interpreterRe = regexp.MustCompile(`^(python|node|nodejs|php|ruby)[0-9.]*$`)
	nonAlnumRe    = regexp.MustCompile(`[^a-z0-9]+`)
	underscoresRe = regexp.MustCompile(`_+`)
)

var mysqlClients = map[string]bool{"mysql": true, "mysqldump": true, "mysqladmin": true}

const blankToken = "{{secret}}"

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
		} else if q := cmd[max(e-1, 0)]; s < e && (q == '"' || q == '\'') &&
			strings.Count(cmd[s:e], string(q)) == 1 && strings.Count(cmd[:s], string(q))%2 == 1 {
			e-- // the quote closing a quoted command: sh -c "app --token=abc"
		}
		if t := strings.Trim(cmd[s:e], `"'`); t == "" || notValueRe.MatchString(t) {
			return
		}
		found = append(found, FoundSecret{Key: key, Value: cmd[s:e], Start: s, End: e})
	}
	wordEnd := func(s int) int {
		if i := strings.IndexAny(cmd[s:], " \t\n\v\f\r"); i >= 0 {
			return s + i
		}
		return len(cmd)
	}
	for _, m := range assignRe.FindAllStringSubmatchIndex(cmd, -1) {
		if key := cmd[m[2]:m[3]]; secretKeyRe.MatchString(key) {
			add(key, m[1], wordEnd(m[1]))
		}
	}
	for _, m := range flagArgRe.FindAllStringSubmatchIndex(cmd, -1) {
		s := m[1]
		for s < len(cmd) && strings.IndexByte(" \t\n\v\f\r", cmd[s]) >= 0 {
			s++
		}
		if key := cmd[m[2]:m[3]]; s > m[1] && s < len(cmd) && cmd[s] != '-' && secretKeyRe.MatchString(key) {
			add(key, s, wordEnd(s))
		}
	}
	for _, m := range urlUserRe.FindAllStringSubmatchIndex(cmd, -1) {
		// Named after the host alone: the user part may be the credential.
		key := "url-" + cmd[m[6]:m[7]]
		if tokenUserRe.MatchString(cmd[m[2]:m[3]]) {
			add(key, m[2], m[3])
		}
		if m[4] >= 0 && cmd[m[4]:m[5]] != "x-oauth-basic" { // GitHub's <token>:x-oauth-basic
			add(key, m[4], m[5])
		}
	}
	// curl -u / --user user:password: the part after the first ":".
	curlUser := func(s, e int) {
		if j := strings.IndexByte(cmd[s:e], ':'); j >= 0 {
			add("curl-password", s+j+1, e)
		}
	}
	mysql, curl := false, false
	words := wordRe.FindAllStringIndex(cmd, -1)
	for i, w := range words {
		word := cmd[w[0]:w[1]]
		prog := path.Base(strings.Trim(word, `"'`))
		switch {
		case mysqlClients[prog]:
			mysql = true
		case prog == "curl":
			curl = true
		case mysql && strings.HasPrefix(word, "-p"):
			add("mysql-password", w[0]+2, w[1])
		case curl && (word == "-u" || word == "--user") && i+1 < len(words):
			curlUser(words[i+1][0], words[i+1][1])
		case curl && strings.HasPrefix(word, "--user="):
			curlUser(w[0]+len("--user="), w[1])
		case curl && strings.HasPrefix(word, "-u"):
			curlUser(w[0]+2, w[1])
		}
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].Start < found[j].Start })
	out, end := found[:0], 0
	for _, f := range found {
		switch {
		case f.Start > end || len(out) == 0:
			out, end = append(out, f), f.End
		case f.End > end:
			// Starts inside the previous value, or right where it ends, and ends
			// past it (a quote closed mid-value): one span covers both, its
			// opening quote too, so no lone quote or pair of tokens is left to
			// re-parse as a value.
			p := &out[len(out)-1]
			if p.Start > 0 && (cmd[p.Start-1] == '"' || cmd[p.Start-1] == '\'') {
				p.Start--
			}
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
// Read with every value blanked: a value never names any of them, and a
// rotated value keeps its identity.
func commandOrigin(cmd string) (group, where, prog string) {
	cmd = redactSpans(cmd, FindSecrets(cmd), blank)
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
		case java || strings.HasPrefix(a, "-") || strings.ContainsAny(a, "={"): // a flag, a setting (php -d k=v), a blanked value
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

func blank(FoundSecret) string { return blankToken }

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
// replaces each value in info by its token (secretNames picks the names), then
// marks info SecretsRedacted. Every value is redacted whatever happens: one the
// vault refused or holds in the trash becomes {{secret}}. Returns how many
// secrets the vault holds for this scan; the error only counts the others.
func RecordScanSecrets(ctx context.Context, db *database.DB, hostID, actorUserID int64, info *sshtest.VMInfo) (int, error) {
	sources, redactOnly := scanCommandLines(info)
	type ident struct{ group, name string }
	values := map[ident]map[string]string{} // value → description
	found := make([][]FoundSecret, len(sources))
	for i, s := range sources {
		found[i] = FindSecrets(*s.cmd)
		for k, f := range found[i] {
			id := ident{s.group, EnvVarName(f.Key)}
			if values[id] == nil {
				values[id] = map[string]string{}
			}
			// Spelled from the line before it, earlier values blanked.
			pre := redactSpans((*s.cmd)[:f.Start], found[i][:k], blank)
			desc := spelling(pre, f.Key) + " · " + s.prog + " · " + s.where
			if old, ok := values[id][f.Value]; !ok || desc < old {
				values[id][f.Value] = desc
			}
		}
	}

	tokens := map[string]string{}  // group, name, value → token
	byValue := map[string]string{} // value → token, for the redact-only lines
	var errs []error
	n := 0
	for id, vals := range values {
		sorted := slices.Sorted(maps.Keys(vals))
		names, err := secretNames(ctx, db, hostID, id.group, id.name, sorted)
		if err != nil {
			errs = append(errs, err)
		}
		for _, v := range sorted {
			tok := blankToken
			if name, ok := names[v]; ok {
				switch sid, err := vault.UpsertScanSecret(ctx, db, hostID, actorUserID, id.group, name, v, vals[v]); {
				case err == nil:
					tok, n = fmt.Sprintf("{{secret:%d}}", sid), n+1
				case !errors.Is(err, vault.ErrScanSecretTrashed):
					errs = append(errs, err)
				}
			}
			tokens[id.group+"\x00"+id.name+"\x00"+v] = tok
			if byValue[v] == "" || byValue[v] == blankToken {
				byValue[v] = tok
			}
		}
	}
	lookup := func(m map[string]string, key string) string {
		if t, ok := m[key]; ok {
			return t
		}
		return blankToken
	}
	for i, s := range sources {
		*s.cmd = redactSpans(*s.cmd, found[i], func(f FoundSecret) string {
			return lookup(tokens, s.group+"\x00"+EnvVarName(f.Key)+"\x00"+f.Value)
		})
	}
	for _, c := range redactOnly {
		*c = redactSpans(*c, FindSecrets(*c), func(f FoundSecret) string { return lookup(byValue, f.Value) })
		if m := cutURLRe.FindStringSubmatchIndex(*c); m != nil && (*c)[m[4]:m[5]] != blankToken &&
			(m[2] >= 0 || !cutHostRe.MatchString((*c)[m[4]:m[5]])) {
			*c = (*c)[:m[4]] + blankToken + (*c)[m[5]:]
		}
	}
	info.SecretsRedacted = true
	if len(errs) > 0 {
		return n, &scanSecretsError{failed: len(errs), err: errors.Join(errs...)}
	}
	return n, nil
}

// scanSecretsError counts what the vault didn't take. Its text names no group
// or secret (both come from the scanned line); the causes stay reachable
// through errors.Is / errors.As.
type scanSecretsError struct {
	failed int
	err    error
}

func (e *scanSecretsError) Error() string {
	return fmt.Sprintf("%d scan secret write(s) failed; their values are blanked in the scan", e.failed)
}

func (e *scanSecretsError) Unwrap() error { return e.err }

// secretNames gives each value of a (group, base) its name: the name already
// holding it (live or trashed — a trashed one stays blank); else a name of
// that family this scan didn't match, in suffix order, so a rotated value
// updates its row; else the next free suffix. A value no longer seen keeps
// its row: a new value never takes a name that still holds a value seen now.
func secretNames(ctx context.Context, db *database.DB, hostID int64, group, base string, values []string) (map[string]string, error) {
	held, names, err := vault.ScanSecretNames(ctx, db, hostID, group, base, values)
	if err != nil {
		return nil, err
	}
	out, used, taken := map[string]string{}, map[string]bool{}, map[string]bool{}
	for _, v := range values {
		if name, ok := held[v]; ok && !used[name] {
			out[v], used[name] = name, true
		}
	}
	free := slices.DeleteFunc(slices.Clone(names), func(name string) bool { return used[name] })
	for _, name := range names {
		taken[name] = true
	}
	next := 1
	for _, v := range values {
		if _, ok := out[v]; ok {
			continue
		}
		if len(free) > 0 {
			out[v], free = free[0], free[1:]
			continue
		}
		for taken[suffixed(base, next)] {
			next++
		}
		out[v], taken[suffixed(base, next)] = suffixed(base, next), true
	}
	return out, nil
}

// suffixed is base for n = 1, else base_<n>.
func suffixed(base string, n int) string {
	if n == 1 {
		return base
	}
	return fmt.Sprintf("%s_%d", base, n)
}

// spelling is the key as the command line wrote it (-DcacheServerPassword):
// the last word of the line before the value.
func spelling(pre, key string) string {
	words := strings.Fields(strings.TrimRight(pre, "=:'\" \t"))
	if len(words) == 0 {
		return key
	}
	return words[len(words)-1]
}

const scanSecretsBackfillFlag = "scan_secrets_backfill_v1"

// scanBackfillBatch is how many host_scans rows the backfill reads at a time.
var scanBackfillBatch = 200

// BackfillScanSecrets registers the secrets of every stored scan after
// afterID, in id order (a later scan's value wins), and rewrites each row that
// held one, redacted and marked secrets_redacted. A marked row (this binary's
// scans) is never read: a redacted line can re-parse to a "value". batchDone,
// when set, gets the last id and the counts so far after each batch. rows is
// the scans rewritten, secrets the secrets found in them, last the id of the
// last scan gone through (afterID when none).
func BackfillScanSecrets(ctx context.Context, db *database.DB, afterID int64, batchDone func(last int64, rows, secrets int)) (rows, secrets int, last int64, err error) {
	ctx = store.WithSystemScope(ctx)
	repo := store.NewHostScanRepo(db.SQL)
	var firstErr error
	failed := 0
	last = afterID
	for {
		batch, err := repo.ListAfter(ctx, last, scanBackfillBatch)
		if err != nil {
			return rows, secrets, last, err
		}
		if len(batch) == 0 {
			break
		}
		for _, s := range batch {
			data, n, recErr := redactScanJSON(ctx, db, s.HostID, s.Data)
			secrets += n
			if errors.Is(recErr, errScanNotRewritten) {
				return rows, secrets, last, recErr
			}
			if recErr != nil {
				if failed++; firstErr == nil {
					firstErr = recErr
				}
			}
			if data != "" {
				if err := repo.UpdateData(ctx, s.ID, data); err != nil {
					return rows, secrets, last, err
				}
				rows++
			}
			last = s.ID
		}
		if batchDone != nil {
			batchDone(last, rows, secrets)
		}
	}
	if failed > 0 {
		return rows, secrets, last, fmt.Errorf("%d scan(s) with a secret the vault didn't take (blanked): %w", failed, firstErr)
	}
	return rows, secrets, last, nil
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
	if info.SecretsRedacted {
		return "", 0, nil
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
		return "", n, fmt.Errorf("%w: %w", errScanNotRewritten, errors.Join(recErr, err))
	}
	if m, ok := tree.(map[string]any); ok {
		m["secrets_redacted"] = true
	}
	out, err := json.Marshal(replaceStrings(tree, repl))
	if err != nil {
		return "", n, fmt.Errorf("%w: %w", errScanNotRewritten, errors.Join(recErr, err))
	}
	return string(out), n, recErr
}

// errScanNotRewritten: a row holding a value could not be rewritten; the
// backfill stops before it so the next start tries it again.
var errScanNotRewritten = errors.New("scan not rewritten")

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

// RunScanSecretsBackfill runs BackfillScanSecrets after the last scan it went
// through (the flag scan_secrets_backfill_v1 holds that id), so a start also
// redacts the plaintext rows an older binary wrote after the previous walk (a
// rolling deploy). The id is saved after every batch, so a start killed
// mid-walk resumes after its last batch; a failure is retried at the next
// start. Logs counts only.
func RunScanSecretsBackfill(ctx context.Context, db *database.DB) {
	settings := store.NewAppSettingsRepo(db.SQL)
	after, _ := strconv.ParseInt(settings.Value(ctx, scanSecretsBackfillFlag), 10, 64)
	saved := after
	save := func(last int64) {
		if last <= saved {
			return
		}
		if err := settings.Set(ctx, scanSecretsBackfillFlag, strconv.FormatInt(last, 10)); err != nil {
			log.Printf("[scan] secrets backfill: save progress: %v", err)
			return
		}
		saved = last
	}
	rows, secrets, last, err := BackfillScanSecrets(ctx, db, after, func(last int64, rows, secrets int) {
		save(last)
		log.Printf("[scan] secrets backfill: batch done, %d scan(s) rewritten, %d secret(s) found so far", rows, secrets)
	})
	save(last) // stopped mid-batch: the scans before the failed one are done
	if err != nil {
		log.Printf("[scan] secrets backfill incomplete, resumed at the next start: %d scan(s) rewritten, %d secret(s) found: %v", rows, secrets, err)
		return
	}
	if rows > 0 {
		log.Printf("[scan] secrets backfill: %d scan(s) rewritten, %d secret(s) found", rows, secrets)
	}
}
