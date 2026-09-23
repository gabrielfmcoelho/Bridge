package coolify

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDomainRefs(t *testing.T) {
	var apps []Application
	var svcs []Service
	if err := json.Unmarshal([]byte(`[
		{"name":"bridge","fqdn":"http://Bridge.x.gov.br/api,https://bridge.x.gov.br","destination":{"server":{"uuid":"m","ip":"host.docker.internal"}}},
		{"name":"api","fqdn":"http://api.x.gov.br:8080","destination":{"server":{"uuid":"s1","ip":"10.0.0.2"}}},
		{"name":"nofqdn","fqdn":null,"destination":{"server":{"uuid":"s1","ip":"10.0.0.2"}}},
		{"name":"sslip","fqdn":"http://abc.10.0.0.2.sslip.io","destination":{"server":{"uuid":"s1","ip":"10.0.0.2"}}}
	]`), &apps); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`[
		{"name":"wiki","server":{"uuid":"s2","ip":"10.0.0.3"},"applications":[{"name":"outline","fqdn":"https://wiki.x.gov.br"},{"name":"db","fqdn":null}]}
	]`), &svcs); err != nil {
		t.Fatal(err)
	}
	got := DomainRefs(apps, svcs, "coolify.x.gov.br")
	want := []DomainRef{
		{Domain: "bridge.x.gov.br", HTTPS: true, ServerUUID: "m", ServerIP: "coolify.x.gov.br", Source: "bridge"},
		{Domain: "api.x.gov.br", ServerUUID: "s1", ServerIP: "10.0.0.2", Source: "api"},
		{Domain: "abc.10.0.0.2.sslip.io", ServerUUID: "s1", ServerIP: "10.0.0.2", Source: "sslip"},
		{Domain: "wiki.x.gov.br", HTTPS: true, ServerUUID: "s2", ServerIP: "10.0.0.3", Source: "wiki"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}
