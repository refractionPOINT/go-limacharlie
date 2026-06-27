package limacharlie

import (
	"fmt"
	"net/url"
)

// aiMemoryHive is the hive that backs per-agent AI memory.
const aiMemoryHive = "ai_memory"

// aiMemoryField is the key under which memory entries are merged.
const aiMemoryField = "memories"

// SetAIMemory upserts a single named memory entry for an agent. The server-side
// merge hook on the ai_memory hive merges this entry into the agent's existing
// record (other entries are preserved). Mirrors the Python SDK AiMemory partial
// set, which posts a partial {"memories": {name: content}} payload to the hive
// data endpoint.
func (org *Organization) SetAIMemory(agent string, name string, content string) error {
	return org.setAIMemoryEntries(agent, map[string]interface{}{name: content})
}

// DeleteAIMemory removes a single named memory entry for an agent by sending a
// null value, which the server-side merge hook treats as a deletion. Other
// entries are preserved. Mirrors the Python SDK AiMemory.delete.
func (org *Organization) DeleteAIMemory(agent string, name string) error {
	return org.setAIMemoryEntries(agent, map[string]interface{}{name: nil})
}

// setAIMemoryEntries posts a partial memories payload (values may be nil to
// delete the matching entry) to the ai_memory hive data endpoint.
func (org *Organization) setAIMemoryEntries(agent string, memories map[string]interface{}) error {
	resp := Dict{}
	path := fmt.Sprintf("hive/%s/%s/%s/data", aiMemoryHive, org.GetOID(), url.PathEscape(agent))
	// GenericPOSTRequest form-encodes the top-level Dict; the nested payload is
	// JSON-marshalled into the "data" form field (nil -> JSON null), matching the
	// Python SDK which sends params={"data": json.dumps({"memories": ...})}.
	return org.GenericPOSTRequest(path, Dict{"data": Dict{aiMemoryField: memories}}, &resp)
}
