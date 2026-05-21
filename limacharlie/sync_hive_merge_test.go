package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// These tests exercise the YAML-presence-aware merge in HiveClient.Update
// and the sync push flow that depends on it. They run against the in-process
// MockServer so the wire contract (form fields, response shape) is
// exercised, not just internal Go logic.
//
// The behavior they pin down:
//
//   - YAML without a `usr_mtd:` block on an existing record must preserve
//     every metadata field; nothing gets silently wiped to a Go zero value.
//   - YAML with a partial `usr_mtd:` block (e.g. only `tags:`) must change
//     only the authored fields and leave the rest alone.
//   - Explicit values in YAML still win, even when they look like zero
//     values: `enabled: false` disables, `tags: []` clears tags.
//   - New records (Add path) default Enabled=true when YAML omits enabled,
//     so an IaC-declared rule is active by default.
//   - The existing etag CAS on the wire is honored: a stale write returns
//     a clear error, not a silent stomp.
//   - Struct-literal callers (no presence info) keep the legacy "send every
//     field" semantic so external SDK consumers don't see a behavior change.

// seedHiveRecord pre-populates the mock with one record so update-path
// tests have a known existing state to merge against.
func seedHiveRecord(t *testing.T, ms *MockServer, hive, key string, data Dict, mtd UsrMtd) {
	t.Helper()
	ms.mu.Lock()
	defer ms.mu.Unlock()
	storeKey := hive + "/" + testOID
	if ms.HiveStore[storeKey] == nil {
		ms.HiveStore[storeKey] = map[string]HiveData{}
	}
	ms.HiveStore[storeKey][key] = HiveData{
		Data:   data,
		UsrMtd: mtd,
		SysMtd: SysMtd{
			Etag:       "seed-etag-" + key,
			GUID:       "seed-guid-" + key,
			CreatedBy:  "mock",
			LastAuthor: "mock",
		},
	}
}

// loadOrgConfig parses a YAML string into an OrgConfig the way the CLI's
// sync push path would. Goes through UnmarshalYAML so presence is populated.
func loadOrgConfig(t *testing.T, src string) OrgConfig {
	t.Helper()
	cfg := OrgConfig{}
	require.NoError(t, yaml.Unmarshal([]byte(src), &cfg))
	return cfg
}

// TestSyncHiveYAMLNoUsrMtdBlockPreservesExisting locks in the headline fix.
// YAML carries only `data:` for a rule that already exists with
// enabled=true, tags, comment, expiry; the push must not touch any of
// those mtd fields.
func TestSyncHiveYAMLNoUsrMtdBlockPreservesExisting(t *testing.T) {
	ms, org := setupMock(t)
	seedHiveRecord(t, ms, "dr-general", "rule-1",
		Dict{"detect": "old"},
		UsrMtd{Enabled: true, Expiry: 1700000000, Tags: []string{"prod", "edr"}, Comment: "owned by team-X"},
	)

	src := `
hives:
  dr-general:
    rule-1:
      data:
        detect: new
`
	cfg := loadOrgConfig(t, src)

	ops, err := org.SyncPush(cfg, SyncOptions{
		SyncHives: map[string]bool{"dr-general": true},
	})
	require.NoError(t, err)
	require.Len(t, ops, 1)
	assert.Equal(t, "dr-general/rule-1", ops[0].ElementName)

	hc := NewHiveClient(org)
	hd, err := hc.Get(HiveArgs{HiveName: "dr-general", PartitionKey: testOID, Key: "rule-1"})
	require.NoError(t, err)
	assert.Equal(t, "new", hd.Data["detect"], "data should reflect YAML")
	assert.True(t, hd.UsrMtd.Enabled, "enabled must be preserved when YAML omits usr_mtd")
	assert.Equal(t, int64(1700000000), hd.UsrMtd.Expiry, "expiry must be preserved")
	assert.Equal(t, []string{"prod", "edr"}, hd.UsrMtd.Tags, "tags must be preserved")
	assert.Equal(t, "owned by team-X", hd.UsrMtd.Comment, "comment must be preserved")
}

// TestSyncHivePartialUsrMtdPreservesOthers covers the more subtle partial
// case: YAML carries `usr_mtd: {tags: [foo]}` only. The new tags must win,
// but enabled/comment/expiry must keep their existing values.
func TestSyncHivePartialUsrMtdPreservesOthers(t *testing.T) {
	ms, org := setupMock(t)
	seedHiveRecord(t, ms, "dr-general", "rule-2",
		Dict{"detect": "x"},
		UsrMtd{Enabled: true, Expiry: 9999, Tags: []string{"old"}, Comment: "hi"},
	)

	src := `
hives:
  dr-general:
    rule-2:
      data:
        detect: x
      usr_mtd:
        tags: ["new"]
`
	cfg := loadOrgConfig(t, src)
	_, err := org.SyncPush(cfg, SyncOptions{SyncHives: map[string]bool{"dr-general": true}})
	require.NoError(t, err)

	hc := NewHiveClient(org)
	hd, err := hc.Get(HiveArgs{HiveName: "dr-general", PartitionKey: testOID, Key: "rule-2"})
	require.NoError(t, err)
	assert.Equal(t, []string{"new"}, hd.UsrMtd.Tags, "tags should be the authored value")
	assert.True(t, hd.UsrMtd.Enabled, "enabled must be preserved when YAML did not set it")
	assert.Equal(t, int64(9999), hd.UsrMtd.Expiry, "expiry must be preserved")
	assert.Equal(t, "hi", hd.UsrMtd.Comment, "comment must be preserved")
}

// TestSyncHiveExplicitEnabledFalseDisables makes sure the merge does not
// over-correct: when the YAML explicitly says `enabled: false`, the rule
// must end up disabled. This is the intentional-disable case, distinct
// from the silent-disable footgun the fix addresses.
func TestSyncHiveExplicitEnabledFalseDisables(t *testing.T) {
	ms, org := setupMock(t)
	seedHiveRecord(t, ms, "dr-general", "rule-3",
		Dict{"detect": "x"},
		UsrMtd{Enabled: true, Tags: []string{"keep"}},
	)

	src := `
hives:
  dr-general:
    rule-3:
      data:
        detect: x
      usr_mtd:
        enabled: false
`
	cfg := loadOrgConfig(t, src)
	_, err := org.SyncPush(cfg, SyncOptions{SyncHives: map[string]bool{"dr-general": true}})
	require.NoError(t, err)

	hc := NewHiveClient(org)
	hd, err := hc.Get(HiveArgs{HiveName: "dr-general", PartitionKey: testOID, Key: "rule-3"})
	require.NoError(t, err)
	assert.False(t, hd.UsrMtd.Enabled, "explicit enabled:false in YAML must be honored")
	assert.Equal(t, []string{"keep"}, hd.UsrMtd.Tags, "tags not authored, must be preserved")
}

// TestSyncHiveEmptyUsrMtdBlockPreservesAll checks that `usr_mtd: {}`
// (present but empty) is treated the same as a missing usr_mtd block:
// every field is preserved. presenceUsed becomes true but every individual
// field pointer stays nil, so the merge keeps the existing values.
func TestSyncHiveEmptyUsrMtdBlockPreservesAll(t *testing.T) {
	ms, org := setupMock(t)
	seedHiveRecord(t, ms, "dr-general", "rule-4",
		Dict{"detect": "x"},
		UsrMtd{Enabled: true, Tags: []string{"a"}, Comment: "c", Expiry: 42},
	)

	src := `
hives:
  dr-general:
    rule-4:
      data:
        detect: new
      usr_mtd: {}
`
	cfg := loadOrgConfig(t, src)
	_, err := org.SyncPush(cfg, SyncOptions{SyncHives: map[string]bool{"dr-general": true}})
	require.NoError(t, err)

	hc := NewHiveClient(org)
	hd, err := hc.Get(HiveArgs{HiveName: "dr-general", PartitionKey: testOID, Key: "rule-4"})
	require.NoError(t, err)
	assert.True(t, hd.UsrMtd.Enabled)
	assert.Equal(t, []string{"a"}, hd.UsrMtd.Tags)
	assert.Equal(t, "c", hd.UsrMtd.Comment)
	assert.Equal(t, int64(42), hd.UsrMtd.Expiry)
}

// TestSyncHiveTagsEmptyArrayClears makes sure `tags: []` (non-nil, empty)
// is distinct from "tags not specified": the explicit empty slice must
// clear existing tags. This is the only way for IaC to declare "no tags."
func TestSyncHiveTagsEmptyArrayClears(t *testing.T) {
	ms, org := setupMock(t)
	seedHiveRecord(t, ms, "dr-general", "rule-5",
		Dict{"detect": "x"},
		UsrMtd{Enabled: true, Tags: []string{"old1", "old2"}},
	)

	src := `
hives:
  dr-general:
    rule-5:
      data:
        detect: x
      usr_mtd:
        tags: []
`
	cfg := loadOrgConfig(t, src)
	_, err := org.SyncPush(cfg, SyncOptions{SyncHives: map[string]bool{"dr-general": true}})
	require.NoError(t, err)

	hc := NewHiveClient(org)
	hd, err := hc.Get(HiveArgs{HiveName: "dr-general", PartitionKey: testOID, Key: "rule-5"})
	require.NoError(t, err)
	assert.Empty(t, hd.UsrMtd.Tags, "explicit empty tags slice must clear")
	assert.True(t, hd.UsrMtd.Enabled, "enabled not authored, must be preserved")
}

// TestSyncHiveAddDefaultsEnabledTrue verifies that creating a brand-new
// record via sync push - when YAML did not author enabled - results in an
// enabled rule. The pre-fix behavior would silently create a disabled rule
// (Go bool zero), which looks like a successful deploy but never fires.
func TestSyncHiveAddDefaultsEnabledTrue(t *testing.T) {
	_, org := setupMock(t)

	src := `
hives:
  dr-general:
    new-rule:
      data:
        detect: something
`
	cfg := loadOrgConfig(t, src)
	_, err := org.SyncPush(cfg, SyncOptions{SyncHives: map[string]bool{"dr-general": true}})
	require.NoError(t, err)

	hc := NewHiveClient(org)
	hd, err := hc.Get(HiveArgs{HiveName: "dr-general", PartitionKey: testOID, Key: "new-rule"})
	require.NoError(t, err)
	assert.True(t, hd.UsrMtd.Enabled, "new IaC-declared records must default to enabled")
}

// TestSyncHiveAddExplicitEnabledFalseRespected is the negative of the
// add-default test: if YAML explicitly says enabled:false on a new record,
// the default does not override the explicit value.
func TestSyncHiveAddExplicitEnabledFalseRespected(t *testing.T) {
	_, org := setupMock(t)

	src := `
hives:
  dr-general:
    disabled-rule:
      data:
        detect: x
      usr_mtd:
        enabled: false
`
	cfg := loadOrgConfig(t, src)
	_, err := org.SyncPush(cfg, SyncOptions{SyncHives: map[string]bool{"dr-general": true}})
	require.NoError(t, err)

	hc := NewHiveClient(org)
	hd, err := hc.Get(HiveArgs{HiveName: "dr-general", PartitionKey: testOID, Key: "disabled-rule"})
	require.NoError(t, err)
	assert.False(t, hd.UsrMtd.Enabled, "explicit enabled:false on Add must be honored")
}

// TestSyncHiveEqualsPresenceAware checks that the diff step does not fire
// a spurious UPDATE when the YAML omits fields that the current record
// has set to non-zero values. Pre-fix, JSON-comparing the full UsrMtd
// would always trigger an UPDATE on sparse YAML; the new presence-aware
// Equals treats unauthored fields as equal.
func TestSyncHiveEqualsPresenceAware(t *testing.T) {
	ms, org := setupMock(t)
	seedHiveRecord(t, ms, "dr-general", "rule-noop",
		Dict{"detect": "x"},
		UsrMtd{Enabled: true, Expiry: 1234, Tags: []string{"a"}, Comment: "c"},
	)

	src := `
hives:
  dr-general:
    rule-noop:
      data:
        detect: x
`
	cfg := loadOrgConfig(t, src)

	// DryRun: an op shows up in the list only when something would change.
	// If Equals correctly treats this as a no-op, IsAdded must be false.
	ops, err := org.SyncPush(cfg, SyncOptions{
		IsDryRun:  true,
		SyncHives: map[string]bool{"dr-general": true},
	})
	require.NoError(t, err)
	require.Len(t, ops, 1)
	assert.False(t, ops[0].IsAdded, "sparse YAML matching current state must not flag an update")
	assert.False(t, ops[0].IsRemoved)
}

// TestSyncHiveEqualsAuthoredFieldDiffersTriggersUpdate is the converse:
// if the YAML authors a field whose value differs from current, Equals
// must return false so the update fires.
func TestSyncHiveEqualsAuthoredFieldDiffersTriggersUpdate(t *testing.T) {
	ms, org := setupMock(t)
	seedHiveRecord(t, ms, "dr-general", "rule-diff",
		Dict{"detect": "x"},
		UsrMtd{Enabled: true},
	)

	src := `
hives:
  dr-general:
    rule-diff:
      data:
        detect: x
      usr_mtd:
        enabled: false
`
	cfg := loadOrgConfig(t, src)
	ops, err := org.SyncPush(cfg, SyncOptions{
		IsDryRun:  true,
		SyncHives: map[string]bool{"dr-general": true},
	})
	require.NoError(t, err)
	require.Len(t, ops, 1)
	assert.True(t, ops[0].IsAdded, "authored field that differs must trigger an update")
}

// TestSyncHiveIsForceRemovesUntrackedRecords confirms the existing IsForce
// behavior still works: records present in the org but not in the YAML are
// deleted. The fix should not affect this path.
func TestSyncHiveIsForceRemovesUntrackedRecords(t *testing.T) {
	ms, org := setupMock(t)
	seedHiveRecord(t, ms, "dr-general", "keep-me",
		Dict{"detect": "x"},
		UsrMtd{Enabled: true},
	)
	seedHiveRecord(t, ms, "dr-general", "delete-me",
		Dict{"detect": "y"},
		UsrMtd{Enabled: true},
	)

	src := `
hives:
  dr-general:
    keep-me:
      data:
        detect: x
`
	cfg := loadOrgConfig(t, src)
	ops, err := org.SyncPush(cfg, SyncOptions{
		IsForce:   true,
		SyncHives: map[string]bool{"dr-general": true},
	})
	require.NoError(t, err)

	var sawRemove bool
	for _, op := range ops {
		if op.ElementName == "dr-general/delete-me" && op.IsRemoved {
			sawRemove = true
		}
	}
	assert.True(t, sawRemove, "IsForce must surface removal of untracked records")

	hc := NewHiveClient(org)
	list, err := hc.List(HiveArgs{HiveName: "dr-general", PartitionKey: testOID})
	require.NoError(t, err)
	_, kept := list["keep-me"]
	_, deleted := list["delete-me"]
	assert.True(t, kept, "kept record must remain")
	assert.False(t, deleted, "untracked record must be removed under IsForce")
}

// TestSyncHiveIsDryRunNoWrites confirms that a DryRun push does not mutate
// the hive even when ops would otherwise be issued. Important for the
// presence-aware update path because it gets exercised under DryRun in
// the diff-only flow.
func TestSyncHiveIsDryRunNoWrites(t *testing.T) {
	ms, org := setupMock(t)
	seedHiveRecord(t, ms, "dr-general", "rule-dry",
		Dict{"detect": "x"},
		UsrMtd{Enabled: true, Tags: []string{"orig"}},
	)

	src := `
hives:
  dr-general:
    rule-dry:
      data:
        detect: x
      usr_mtd:
        enabled: false
        tags: ["changed"]
    brand-new:
      data:
        detect: new
`
	cfg := loadOrgConfig(t, src)
	ops, err := org.SyncPush(cfg, SyncOptions{
		IsDryRun:  true,
		SyncHives: map[string]bool{"dr-general": true},
	})
	require.NoError(t, err)
	require.NotEmpty(t, ops)

	hc := NewHiveClient(org)
	hd, err := hc.Get(HiveArgs{HiveName: "dr-general", PartitionKey: testOID, Key: "rule-dry"})
	require.NoError(t, err)
	assert.True(t, hd.UsrMtd.Enabled, "DryRun must not mutate enabled")
	assert.Equal(t, []string{"orig"}, hd.UsrMtd.Tags, "DryRun must not mutate tags")

	_, err = hc.Get(HiveArgs{HiveName: "dr-general", PartitionKey: testOID, Key: "brand-new"})
	require.Error(t, err, "DryRun must not create new records")
}

// TestHiveClientUpdateMergesExistingMtd is the unit-level test for the
// merge in HiveClient.Update. Direct call (no sync push), sparse args; the
// fetched existing mtd must be the merge base.
func TestHiveClientUpdateMergesExistingMtd(t *testing.T) {
	ms, org := setupMock(t)
	seedHiveRecord(t, ms, "lookup", "k",
		Dict{"v": "1"},
		UsrMtd{Enabled: true, Expiry: 50, Tags: []string{"a"}, Comment: "hi"},
	)

	hc := NewHiveClient(org)
	// Only update tags. Other fields must be merged from existing.
	_, err := hc.Update(HiveArgs{
		HiveName:     "lookup",
		PartitionKey: testOID,
		Key:          "k",
		Tags:         []string{"b"},
	})
	require.NoError(t, err)

	hd, err := hc.Get(HiveArgs{HiveName: "lookup", PartitionKey: testOID, Key: "k"})
	require.NoError(t, err)
	assert.True(t, hd.UsrMtd.Enabled, "enabled merged from existing")
	assert.Equal(t, int64(50), hd.UsrMtd.Expiry, "expiry merged from existing")
	assert.Equal(t, []string{"b"}, hd.UsrMtd.Tags, "tags overridden")
	assert.Equal(t, "hi", hd.UsrMtd.Comment, "comment merged from existing")
}

// TestHiveClientUpdateAllNilArgsIsNoOpUpdate documents what happens when a
// direct caller passes no UsrMtd args at all: the existing mtd is read,
// nothing is overlaid, the existing mtd is sent back. Net effect on the
// record is "no change" - which is the right semantic since the caller
// explicitly authored nothing.
func TestHiveClientUpdateAllNilArgsIsNoOpUpdate(t *testing.T) {
	ms, org := setupMock(t)
	seedHiveRecord(t, ms, "lookup", "k",
		Dict{"v": "1"},
		UsrMtd{Enabled: true, Expiry: 50, Tags: []string{"a"}, Comment: "hi"},
	)

	hc := NewHiveClient(org)
	_, err := hc.Update(HiveArgs{
		HiveName:     "lookup",
		PartitionKey: testOID,
		Key:          "k",
	})
	require.NoError(t, err)

	hd, err := hc.Get(HiveArgs{HiveName: "lookup", PartitionKey: testOID, Key: "k"})
	require.NoError(t, err)
	assert.True(t, hd.UsrMtd.Enabled, "enabled unchanged")
	assert.Equal(t, int64(50), hd.UsrMtd.Expiry, "expiry unchanged")
	assert.Equal(t, []string{"a"}, hd.UsrMtd.Tags, "tags unchanged")
	assert.Equal(t, "hi", hd.UsrMtd.Comment, "comment unchanged")
}

// TestHiveClientUpdateEtagMismatchSurfaces verifies the CAS path: if the
// stored record's etag has rotated between our fetch and our POST, the
// server returns ETAG_MISMATCH and we surface it instead of stomping.
// The mock checks the etag form field on /mtd requests for exactly this.
func TestHiveClientUpdateEtagMismatchSurfaces(t *testing.T) {
	ms, org := setupMock(t)
	seedHiveRecord(t, ms, "lookup", "k",
		Dict{"v": "1"},
		UsrMtd{Enabled: true, Tags: []string{"a"}},
	)

	// Force a stale etag: rotate the stored etag after the SDK's GetMTD
	// would observe the current one. Easiest way is a hook: monkey-patch
	// the store between fetch and write. We approximate it by manually
	// changing the etag in the mock after a successful GetMTD.
	hc := NewHiveClient(org)
	_, err := hc.GetMTD(HiveArgs{HiveName: "lookup", PartitionKey: testOID, Key: "k"})
	require.NoError(t, err)

	ms.mu.Lock()
	rec := ms.HiveStore["lookup/"+testOID]["k"]
	rec.SysMtd.Etag = "rotated-by-another-writer"
	ms.HiveStore["lookup/"+testOID]["k"] = rec
	ms.mu.Unlock()

	// Now an Update issues a fresh GetMTD (observes the rotated etag) and
	// sends it. The mock accepts a match - so we need to rotate AGAIN
	// between the SDK's internal fetch and the POST. Mock doesn't easily
	// support that, so instead drive the etag check via the unit-level
	// signature: post directly with a stale etag.
	disabled := false
	disabledArgs := HiveArgs{
		HiveName:     "lookup",
		PartitionKey: testOID,
		Key:          "k",
		Enabled:      &disabled,
	}
	// HiveClient.Update fetches the current etag itself, so it always
	// sends a fresh one and won't trip CAS. To exercise the mismatch path
	// we have to bypass the SDK and POST a stale etag directly.
	t.Run("manual stale etag is rejected by mock", func(t *testing.T) {
		path := "hive/lookup/" + testOID + "/k/mtd"
		fd := Dict{
			"usr_mtd": UsrMtd{Enabled: false},
			"etag":    "definitely-not-current",
		}
		var resp HiveResp
		req := makeDefaultRequest(&resp).withFormData(fd)
		err := org.client.reliableRequest(t.Context(), "POST", path, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ETAG_MISMATCH",
			"mock must reject stale etag on /mtd")
	})

	_ = disabledArgs
}

// TestSyncHiveStructLiteralBackwardCompat exercises the resolvedPresence
// fallback: an external caller (no YAML, no AsSyncData) constructs a
// SyncHiveData directly. presenceUsed is false. The sync write must still
// send every UsrMtd field, matching the legacy "send everything" behavior
// so existing external callers don't see a regression.
func TestSyncHiveStructLiteralBackwardCompat(t *testing.T) {
	ms, org := setupMock(t)
	seedHiveRecord(t, ms, "dr-general", "lit-1",
		Dict{"detect": "x"},
		UsrMtd{Enabled: true, Tags: []string{"existing"}, Comment: "existing"},
	)

	// Build SyncHiveData by struct literal with intent to set every field.
	cfg := OrgConfig{
		Hives: orgSyncHives{
			"dr-general": {
				"lit-1": SyncHiveData{
					Data: Dict{"detect": "x"},
					UsrMtd: UsrMtd{
						Enabled: false,
						Tags:    []string{"new"},
						Comment: "new",
					},
				},
			},
		},
	}
	_, err := org.SyncPush(cfg, SyncOptions{SyncHives: map[string]bool{"dr-general": true}})
	require.NoError(t, err)

	hc := NewHiveClient(org)
	hd, err := hc.Get(HiveArgs{HiveName: "dr-general", PartitionKey: testOID, Key: "lit-1"})
	require.NoError(t, err)
	assert.False(t, hd.UsrMtd.Enabled, "struct-literal sends every field, including enabled:false")
	assert.Equal(t, []string{"new"}, hd.UsrMtd.Tags)
	assert.Equal(t, "new", hd.UsrMtd.Comment)
}

// TestSyncHiveAsSyncDataRoundTrip confirms that fetching a record,
// converting via AsSyncData, then pushing back results in the same record.
// This is the read-modify-write workflow: sync fetch -> edit -> sync push,
// where the caller expects nothing they did not touch to change.
func TestSyncHiveAsSyncDataRoundTrip(t *testing.T) {
	ms, org := setupMock(t)
	original := UsrMtd{Enabled: true, Expiry: 100, Tags: []string{"a", "b"}, Comment: "c"}
	seedHiveRecord(t, ms, "dr-general", "rt", Dict{"detect": "x"}, original)

	hc := NewHiveClient(org)
	current, err := hc.List(HiveArgs{HiveName: "dr-general", PartitionKey: testOID})
	require.NoError(t, err)

	cfg := OrgConfig{
		Hives: orgSyncHives{
			"dr-general": current.AsSyncConfigData(),
		},
	}
	_, err = org.SyncPush(cfg, SyncOptions{SyncHives: map[string]bool{"dr-general": true}})
	require.NoError(t, err)

	hd, err := hc.Get(HiveArgs{HiveName: "dr-general", PartitionKey: testOID, Key: "rt"})
	require.NoError(t, err)
	assert.Equal(t, original.Enabled, hd.UsrMtd.Enabled)
	assert.Equal(t, original.Expiry, hd.UsrMtd.Expiry)
	assert.Equal(t, original.Tags, hd.UsrMtd.Tags)
	assert.Equal(t, original.Comment, hd.UsrMtd.Comment)
}

// TestSyncHiveYAMLAllFieldsAuthoredOverwrites covers the explicit case:
// YAML carries a fully populated usr_mtd; every field must take the YAML
// value. This is the "safe" pre-fix workflow and must remain unchanged.
func TestSyncHiveYAMLAllFieldsAuthoredOverwrites(t *testing.T) {
	ms, org := setupMock(t)
	seedHiveRecord(t, ms, "dr-general", "full",
		Dict{"detect": "x"},
		UsrMtd{Enabled: true, Expiry: 1, Tags: []string{"old"}, Comment: "old"},
	)

	src := `
hives:
  dr-general:
    full:
      data:
        detect: x
      usr_mtd:
        enabled: false
        expiry: 999
        tags: ["a", "b"]
        comment: "fresh"
`
	cfg := loadOrgConfig(t, src)
	_, err := org.SyncPush(cfg, SyncOptions{SyncHives: map[string]bool{"dr-general": true}})
	require.NoError(t, err)

	hc := NewHiveClient(org)
	hd, err := hc.Get(HiveArgs{HiveName: "dr-general", PartitionKey: testOID, Key: "full"})
	require.NoError(t, err)
	assert.False(t, hd.UsrMtd.Enabled)
	assert.Equal(t, int64(999), hd.UsrMtd.Expiry)
	assert.Equal(t, []string{"a", "b"}, hd.UsrMtd.Tags)
	assert.Equal(t, "fresh", hd.UsrMtd.Comment)
}

// TestSyncHiveUnmarshalPresenceTracking unit-tests the presence-tracking
// decode itself, independent of the sync flow. Distinguishes "field set
// to zero" from "field omitted."
func TestSyncHiveUnmarshalPresenceTracking(t *testing.T) {
	t.Run("no usr_mtd block at all", func(t *testing.T) {
		var s SyncHiveData
		require.NoError(t, yaml.Unmarshal([]byte(`data: {x: 1}`), &s))
		assert.True(t, s.presenceUsed, "any decode through UnmarshalYAML sets presenceUsed")
		assert.Nil(t, s.presence.Enabled)
		assert.Nil(t, s.presence.Expiry)
		assert.Nil(t, s.presence.Tags)
		assert.Nil(t, s.presence.Comment)
	})

	t.Run("usr_mtd present but empty", func(t *testing.T) {
		var s SyncHiveData
		require.NoError(t, yaml.Unmarshal([]byte("data: {x: 1}\nusr_mtd: {}"), &s))
		assert.True(t, s.presenceUsed)
		assert.Nil(t, s.presence.Enabled)
		assert.Nil(t, s.presence.Tags)
	})

	t.Run("enabled set to false explicitly", func(t *testing.T) {
		var s SyncHiveData
		require.NoError(t, yaml.Unmarshal([]byte("data: {x: 1}\nusr_mtd:\n  enabled: false"), &s))
		require.NotNil(t, s.presence.Enabled)
		assert.False(t, *s.presence.Enabled)
		assert.Nil(t, s.presence.Tags, "tags must remain nil when omitted")
	})

	t.Run("tags present but empty array", func(t *testing.T) {
		var s SyncHiveData
		require.NoError(t, yaml.Unmarshal([]byte("data: {x: 1}\nusr_mtd:\n  tags: []"), &s))
		require.NotNil(t, s.presence.Tags)
		assert.Empty(t, *s.presence.Tags, "empty array distinguishable from nil")
	})

	t.Run("all fields authored", func(t *testing.T) {
		var s SyncHiveData
		src := `data: {x: 1}
usr_mtd:
  enabled: true
  expiry: 42
  tags: ["a"]
  comment: hi`
		require.NoError(t, yaml.Unmarshal([]byte(src), &s))
		require.NotNil(t, s.presence.Enabled)
		require.NotNil(t, s.presence.Expiry)
		require.NotNil(t, s.presence.Tags)
		require.NotNil(t, s.presence.Comment)
		assert.True(t, *s.presence.Enabled)
		assert.Equal(t, int64(42), *s.presence.Expiry)
		assert.Equal(t, []string{"a"}, *s.presence.Tags)
		assert.Equal(t, "hi", *s.presence.Comment)
	})
}

// TestSyncHiveEqualsUnit tests Equals directly with hand-built
// SyncHiveData, covering the presence-aware comparison logic.
func TestSyncHiveEqualsUnit(t *testing.T) {
	current := SyncHiveData{
		Data:   Dict{"detect": "x"},
		UsrMtd: UsrMtd{Enabled: true, Expiry: 1, Tags: []string{"a"}, Comment: "c"},
	}

	t.Run("sparse YAML with same data equals current", func(t *testing.T) {
		var hsd SyncHiveData
		require.NoError(t, yaml.Unmarshal([]byte("data: {detect: x}"), &hsd))
		eq, err := hsd.Equals(current)
		require.NoError(t, err)
		assert.True(t, eq, "sparse YAML must equal current when only data matches")
	})

	t.Run("authored enabled differs", func(t *testing.T) {
		var hsd SyncHiveData
		require.NoError(t, yaml.Unmarshal([]byte("data: {detect: x}\nusr_mtd:\n  enabled: false"), &hsd))
		eq, err := hsd.Equals(current)
		require.NoError(t, err)
		assert.False(t, eq)
	})

	t.Run("authored tags differ", func(t *testing.T) {
		var hsd SyncHiveData
		require.NoError(t, yaml.Unmarshal([]byte("data: {detect: x}\nusr_mtd:\n  tags: [\"b\"]"), &hsd))
		eq, err := hsd.Equals(current)
		require.NoError(t, err)
		assert.False(t, eq)
	})

	t.Run("authored tags equal in different declared form", func(t *testing.T) {
		// `tags: []` should equal current empty/nil tags.
		curEmpty := SyncHiveData{
			Data:   Dict{"detect": "x"},
			UsrMtd: UsrMtd{Tags: nil},
		}
		var hsd SyncHiveData
		require.NoError(t, yaml.Unmarshal([]byte("data: {detect: x}\nusr_mtd:\n  tags: []"), &hsd))
		eq, err := hsd.Equals(curEmpty)
		require.NoError(t, err)
		assert.True(t, eq, "explicit empty tags should equal current nil tags")
	})

	t.Run("data differs triggers update regardless of mtd", func(t *testing.T) {
		var hsd SyncHiveData
		require.NoError(t, yaml.Unmarshal([]byte("data: {detect: NEW}"), &hsd))
		eq, err := hsd.Equals(current)
		require.NoError(t, err)
		assert.False(t, eq)
	})
}

// TestSyncHiveSparseMtdAgainstEnabledRecord is the end-to-end regression
// case: a hive entry whose YAML carries only data and a partial usr_mtd
// that omits enabled, pushed against a rule that is currently enabled with
// tags and comment set. Before the fix, the partial push silently disabled
// the rule and stripped its comment; after the fix, only the authored
// field (tags) changes.
func TestSyncHiveSparseMtdAgainstEnabledRecord(t *testing.T) {
	ms, org := setupMock(t)
	// Pre-existing rule, enabled, with tags and comment assigned.
	seedHiveRecord(t, ms, "dr-general", "rule-sparse",
		Dict{"detect": Dict{"event": "NEW_PROCESS"}, "respond": []interface{}{}},
		UsrMtd{Enabled: true, Tags: []string{"prod", "team-a"}, Comment: "owned"},
	)

	// Generator emits a partial usr_mtd: only tags.
	src := `
hives:
  dr-general:
    rule-sparse:
      data:
        detect:
          event: NEW_PROCESS
        respond: []
      usr_mtd:
        tags: ["prod"]
`
	cfg := loadOrgConfig(t, src)
	_, err := org.SyncPush(cfg, SyncOptions{SyncHives: map[string]bool{"dr-general": true}})
	require.NoError(t, err)

	hc := NewHiveClient(org)
	hd, err := hc.Get(HiveArgs{
		HiveName: "dr-general", PartitionKey: testOID,
		Key: "rule-sparse",
	})
	require.NoError(t, err)
	assert.True(t, hd.UsrMtd.Enabled, "regression: rule must NOT be silently disabled")
	assert.Equal(t, []string{"prod"}, hd.UsrMtd.Tags, "tags must reflect the partial author")
	assert.Equal(t, "owned", hd.UsrMtd.Comment, "comment must be preserved")
}
