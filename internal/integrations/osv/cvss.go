package osv

import (
	"math"
	"strings"
)

// CVSS3Score computes the CVSS 3.x base score of a vector
// ("CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H" → 9.8), per the 3.1
// specification. ok is false for anything that isn't a full 3.x vector.
func CVSS3Score(vector string) (score float64, ok bool) {
	parts := strings.Split(vector, "/")
	if len(parts) < 9 || !strings.HasPrefix(parts[0], "CVSS:3") {
		return 0, false
	}
	m := map[string]string{}
	for _, p := range parts[1:] {
		k, v, found := strings.Cut(p, ":")
		if found {
			m[k] = v
		}
	}
	scope := m["S"]
	weights := map[string]map[string]float64{
		"AV": {"N": 0.85, "A": 0.62, "L": 0.55, "P": 0.2},
		"AC": {"L": 0.77, "H": 0.44},
		"UI": {"N": 0.85, "R": 0.62},
		"C":  {"H": 0.56, "L": 0.22, "N": 0},
		"I":  {"H": 0.56, "L": 0.22, "N": 0},
		"A":  {"H": 0.56, "L": 0.22, "N": 0},
	}
	pr := map[string]float64{"N": 0.85, "L": 0.62, "H": 0.27}
	if scope == "C" {
		pr = map[string]float64{"N": 0.85, "L": 0.68, "H": 0.5}
	}
	get := func(metric string, table map[string]float64) (float64, bool) {
		v, found := table[m[metric]]
		return v, found
	}
	av, ok1 := get("AV", weights["AV"])
	ac, ok2 := get("AC", weights["AC"])
	ui, ok3 := get("UI", weights["UI"])
	c, ok4 := get("C", weights["C"])
	i, ok5 := get("I", weights["I"])
	a, ok6 := get("A", weights["A"])
	p, ok7 := get("PR", pr)
	if !(ok1 && ok2 && ok3 && ok4 && ok5 && ok6 && ok7) || (scope != "U" && scope != "C") {
		return 0, false
	}
	iss := 1 - (1-c)*(1-i)*(1-a)
	var impact float64
	if scope == "U" {
		impact = 6.42 * iss
	} else {
		impact = 7.52*(iss-0.029) - 3.25*math.Pow(iss-0.02, 15)
	}
	if impact <= 0 {
		return 0, true
	}
	exploit := 8.22 * av * ac * p * ui
	if scope == "U" {
		return roundUp(math.Min(impact+exploit, 10)), true
	}
	return roundUp(math.Min(1.08*(impact+exploit), 10)), true
}

// roundUp is the spec's Roundup: the smallest one-decimal number >= x,
// computed on integers to dodge float noise.
func roundUp(x float64) float64 {
	n := int64(math.Round(x * 100000))
	if n%10000 == 0 {
		return float64(n) / 100000
	}
	return float64(n/10000+1) / 10
}
