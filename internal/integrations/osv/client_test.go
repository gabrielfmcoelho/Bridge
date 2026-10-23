package osv

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCVSS3Score(t *testing.T) {
	for vec, want := range map[string]float64{
		"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H": 9.8,
		"CVSS:3.1/AV:N/AC:L/PR:N/UI:R/S:C/C:L/I:L/A:N": 6.1,
		"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:L": 5.3,
		"CVSS:3.1/AV:N/AC:H/PR:N/UI:N/S:U/C:N/I:N/A:H": 5.9,
		"CVSS:3.0/AV:L/AC:L/PR:L/UI:N/S:U/C:H/I:H/A:H": 7.8,
		"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:N": 0,
	} {
		got, ok := CVSS3Score(vec)
		if !ok || got != want {
			t.Errorf("CVSS3Score(%s) = %v %v, want %v", vec, got, ok, want)
		}
	}
	for _, bad := range []string{"", "CVSS:2.0/AV:N", "CVSS:3.1/AV:X/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H"} {
		if _, ok := CVSS3Score(bad); ok {
			t.Errorf("CVSS3Score(%q) accepted", bad)
		}
	}
}

func TestQueryBatch_ChunksPagesAndOrder(t *testing.T) {
	var posts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts++
		var body struct {
			Queries []batchQuery `json:"queries"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		w.Write([]byte(`{"results":[`))
		for i, q := range body.Queries {
			if i > 0 {
				w.Write([]byte(","))
			}
			switch {
			case q.Package.Name == "openssl" && q.PageToken == "":
				w.Write([]byte(`{"vulns":[{"id":"DEBIAN-CVE-1"}],"next_page_token":"t2"}`))
			case q.Package.Name == "openssl":
				w.Write([]byte(`{"vulns":[{"id":"DEBIAN-CVE-2"}]}`))
			default:
				fmt.Fprintf(w, `{"vulns":[]}`)
			}
		}
		w.Write([]byte(`]}`))
	}))
	defer srv.Close()
	qs := make([]Query, 1002)
	for i := range qs {
		qs[i] = Query{Ecosystem: "Debian:12", Name: fmt.Sprintf("pkg%d", i), Version: "1"}
	}
	qs[1001].Name = "openssl"
	got, err := (&Client{BaseURL: srv.URL, HTTP: srv.Client()}).QueryBatch(context.Background(), qs)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1002 || len(got[0]) != 0 || fmt.Sprint(got[1001]) != "[DEBIAN-CVE-1 DEBIAN-CVE-2]" {
		t.Fatalf("got[1001] = %v (len %d)", got[1001], len(got))
	}
	if posts != 3 { // chunk 1 (1000), chunk 2 (2), chunk 2 page 2
		t.Fatalf("posts = %d, want 3", posts)
	}
}

func TestFromRecord(t *testing.T) {
	var rec vulnRecord
	json.Unmarshal([]byte(`{"id":"UBUNTU-CVE-2023-2975","details":"AES-SIV bug","upstream":["CVE-2023-2975"],
		"severity":[{"type":"CVSS_V3","score":"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:L/A:N"},{"type":"Ubuntu","score":"low"}],
		"affected":[{"package":{"name":"openssl","ecosystem":"Ubuntu:22.04:LTS"},"ranges":[{"events":[{"introduced":"0"},{"fixed":"3.0.2-0ubuntu1.12"}]}]},
		            {"package":{"name":"nodejs","ecosystem":"Ubuntu:Pro:22.04:LTS"},"ranges":[{"events":[{"introduced":"0"},{"fixed":"9"}]}]}]}`), &rec)
	v := fromRecord(rec)
	if v.CVE != "CVE-2023-2975" || v.CVSS != 5.3 || v.Severity != "medium" || v.Summary != "AES-SIV bug" {
		t.Fatalf("vuln = %+v", v)
	}
	if f := v.FixedFor("Ubuntu:22.04", "openssl"); f != "3.0.2-0ubuntu1.12" {
		t.Fatalf("fixed = %q", f)
	}
	if f := v.FixedFor("Ubuntu:22.04", "nodejs"); f != "" {
		t.Fatalf("Pro stream leaked: %q", f)
	}
	var low vulnRecord
	json.Unmarshal([]byte(`{"id":"UBUNTU-CVE-X","severity":[{"type":"Ubuntu","score":"High"}]}`), &low)
	if v := fromRecord(low); v.Severity != "high" || v.CVE != "" {
		t.Fatalf("priority fallback = %+v", v)
	}
}
