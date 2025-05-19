package common_conf

type ArtifactConfiguration struct {
	Rules    map[string]ArtifactRuleSet    `json:"log_rules"`
	Captures map[string]ArtifactCaptureSet `json:"capture_rules"`
}

type ArtifactRuleSet struct {
	Filters       ArtifactFilter `json:"filters"`
	Patterns      []string       `json:"patterns"`
	DaysRetention int            `json:"days_retention"`
	DeleteAfter   bool           `json:"is_delete_after"`
	IgnoreCert    bool           `json:"is_ignore_cert"`
}

type ArtifactCaptureSet struct {
	Filters       ArtifactFilter           `json:"filters"`
	Patterns      []ArtifactCapturePattern `json:"patterns"`
	DaysRetention int                      `json:"days_retention"`
}

type ArtifactFilter struct {
	Platforms []string `json:"platforms"`
	Tags      []string `json:"tags"`
}

type ArtifactCapturePattern struct {
	Interface string `json:"iface"`
	Filter    string `json:"filter"`
}
