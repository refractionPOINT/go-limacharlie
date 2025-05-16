package common_conf

type ExfilConfiguration struct {
	Rules ExfilRuleSet `json:"exfil_rules"`
}

type ExfilRuleSet struct {
	List  map[string]ExfilRule `json:"list"`
	Watch map[string]ExfilRule `json:"watch"`
	Perf  map[string]ExfilRule `json:"perf"`
}

type ExfilRule struct {
	Filters  ExfilFilter `json:"filters"`
	Events   []string    `json:"events"`
	Name     string      `json:"name"`
	Event    string      `json:"event"`
	Operator string      `json:"operator"`
	Value    string      `json:"value"`
	Path     []string    `json:"path"`
	Tags     []string    `json:"tags"`
	Updated  int64       `json:"updated"`
}

type ExfilFilter struct {
	Platforms []string `json:"platforms"`
	Tags      []string `json:"tags"`
	Updated   int64    `json:"updated"`
}
