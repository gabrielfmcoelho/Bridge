package gitlab

import "testing"

func TestRepoPath(t *testing.T) {
	base := "https://gitlab.sead.pi.gov.br"
	for raw, want := range map[string]string{
		"git@gitlab.sead.pi.gov.br:sead/app.git":                    "sead/app",
		"ssh://git@gitlab.sead.pi.gov.br:2222/sead/sub/app.git":     "sead/sub/app",
		"https://gitlab.sead.pi.gov.br/sead/app":                    "sead/app",
		"https://gitlab.sead.pi.gov.br/sead/app/-/tree/main/docker": "sead/app",
		"https://github.com/sead/app":                               "", // another host
		"https://gitlab.sead.pi.gov.br/sead":                        "", // a group, not a repo
		"":                                                          "",
	} {
		got, ok := RepoPath(raw, base)
		if got != want || ok != (want != "") {
			t.Errorf("RepoPath(%q) = %q, %v; want %q", raw, got, ok, want)
		}
	}
}
