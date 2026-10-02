package service

import "testing"

// Rows from the ETIPI inventory of 25/09/2026 (HTTP, HTTPS → its "Status").
func TestObsStatus(t *testing.T) {
	for _, c := range []struct {
		http, https int
		want        string
	}{
		{302, 404, "online"},     // agentes.sead
		{301, 500, "online"},     // api.painel.sead
		{502, 302, "online"},     // deploy.sead
		{404, 503, "no_content"}, // api.agentes.sead
		{503, 0, "error"},        // dashboards.inteligencia.sead
		{502, 502, "error"},      // dev.admin.cellguard
		{0, 0, "offline"},        // biruta.sead
	} {
		if got := obsStatus(c.http, c.https); got != c.want {
			t.Errorf("obsStatus(%d, %d) = %q, want %q", c.http, c.https, got, c.want)
		}
	}
}
