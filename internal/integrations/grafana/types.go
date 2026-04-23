package grafana

// Health is the response from GET /api/health.
type Health struct {
	Commit   string `json:"commit"`
	Database string `json:"database"`
	Version  string `json:"version"`
}

// User is the response from GET /api/user.
type User struct {
	ID    int    `json:"id"`
	Login string `json:"login"`
	Email string `json:"email"`
	Name  string `json:"name"`
	OrgID int    `json:"orgId"`
}

// PrometheusVectorResponse is the standard shape returned by Prometheus's
// HTTP API /api/v1/query endpoint for instant queries. We tunnel through
// Grafana's datasource proxy (/api/datasources/proxy/uid/<uid>/api/v1/query)
// which returns this verbatim.
type PrometheusVectorResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string              `json:"resultType"`
		Result     []PrometheusSample  `json:"result"`
	} `json:"data"`
	ErrorType string `json:"errorType,omitempty"`
	Error     string `json:"error,omitempty"`
}

// PrometheusSample is one entry in a vector response. Value is [timestamp_unix, "value_as_string"].
type PrometheusSample struct {
	Metric map[string]string `json:"metric"`
	Value  [2]any            `json:"value"`
}

// DashboardCreateResponse is returned by POST /api/dashboards/db.
// URL is relative to the Grafana base URL (e.g. "/d/sshcm-host-slug/host-slug").
type DashboardCreateResponse struct {
	ID      int    `json:"id"`
	UID     string `json:"uid"`
	URL     string `json:"url"`
	Slug    string `json:"slug"`
	Status  string `json:"status"`
	Version int    `json:"version"`
}
