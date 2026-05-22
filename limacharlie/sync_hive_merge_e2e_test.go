package limacharlie

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// End-to-end coverage for the usr_mtd merge fix. These tests drive a real
// LimaCharlie API via the standard integration-test pattern (`_OID` and
// `_KEY` environment variables consumed by getTestOrgFromEnv); they are
// run automatically by the cloudbuild.yaml step alongside the rest of the
// integration suite.
//
// Hive choice: every test uses the `lookup` hive. Lookup is pure
// user-data storage with no live side effects (no sensor connection, no
// detection firing, no extension subscription), and its usr_mtd handling
// is identical to dr-general - so the merge contract is exercised
// end-to-end while keeping blast radius near zero.
//
// Concurrency / collision safety: each record key is `e2e-mtd-merge-` +
// a fresh UUID. Two concurrent CI builds against the same test org cannot
// collide, and stale records from a crashed prior run cannot interfere
// with a new run.
//
// Cleanup: every test registers a t.Cleanup that issues a Remove for the
// record(s) it created. The cleanup ignores errors (a record that was
// never created or was already deleted is not a test failure) so test
// status reflects the assertion, not the teardown.
//
// Tests are marked t.Parallel() because they only touch records under
// their own UUID keys.

const e2eMtdMergeHive = "lookup"

// e2eMtdMergeKey returns a unique record key for a single e2e test run.
// The collision space is the full UUID space, so concurrent CI runs and
// stale state cannot accidentally share a key.
func e2eMtdMergeKey() string {
	return "e2e-mtd-merge-" + uuid.New().String()
}

// e2eMtdMergeData builds a lookup-hive-valid data block. The lookup hive
// rejects records whose lookup_data field is empty, so every test that
// creates a record needs at least one indicator entry under lookup_data.
// The indicator key is unique per call so concurrent records don't share
// content (useful only as defense in depth - the record key itself is
// already unique).
func e2eMtdMergeData(indicator string) Dict {
	return Dict{
		"lookup_data": Dict{
			indicator: Dict{},
		},
	}
}

// e2eMtdMergeSetup builds an Organization from env, creates a HiveClient,
// and registers cleanup that removes any test records the caller added
// to the returned key set. The caller appends to keys via addKey.
func e2eMtdMergeSetup(t *testing.T) (org *Organization, hc *HiveClient, addKey func(string)) {
	t.Helper()
	a := assert.New(t)
	org = getTestOrgFromEnv(a)
	hc = NewHiveClient(org)
	var created []string
	addKey = func(k string) {
		created = append(created, k)
	}
	t.Cleanup(func() {
		for _, k := range created {
			_, _ = hc.Remove(HiveArgs{
				HiveName:     e2eMtdMergeHive,
				PartitionKey: org.GetOID(),
				Key:          k,
			})
		}
	})
	return org, hc, addKey
}

// e2eMtdMergeSeed adds a record to the lookup hive with a full usr_mtd
// block. Returns once the record is present and visible (the SDK's
// reliableRequest blocks until the server responds 200).
func e2eMtdMergeSeed(t *testing.T, hc *HiveClient, oid, key string, data Dict, mtd UsrMtd) {
	t.Helper()
	enabled := mtd.Enabled
	expiry := mtd.Expiry
	comment := mtd.Comment
	_, err := hc.Add(HiveArgs{
		HiveName:     e2eMtdMergeHive,
		PartitionKey: oid,
		Key:          key,
		Data:         data,
		Enabled:      &enabled,
		Expiry:       &expiry,
		Tags:         mtd.Tags,
		Comment:      &comment,
	})
	require.NoError(t, err, "seed Add for key %q must succeed", key)
}

// e2eMtdMergePush invokes SyncPush against the lookup hive with
// IsForce=false. IsForce is intentionally off: a force push against a
// real org's lookup hive would delete every record not present in the
// test YAML, which would be a destructive side effect on whatever else
// happens to live in the lookup hive of the test org.
func e2eMtdMergePush(t *testing.T, org *Organization, yamlSrc string) {
	t.Helper()
	var cfg OrgConfig
	require.NoError(t, yaml.Unmarshal([]byte(yamlSrc), &cfg), "test YAML must parse")
	_, err := org.SyncPush(cfg, SyncOptions{
		SyncHives: map[string]bool{e2eMtdMergeHive: true},
	})
	require.NoError(t, err, "SyncPush against lookup hive must succeed")
}

// TestE2ESyncHive_NoUsrMtdBlock_PreservesAll is the headline regression
// test. A YAML push that carries only `data:` for an existing record
// must not change enabled/expiry/tags/comment.
func TestE2ESyncHive_NoUsrMtdBlock_PreservesAll(t *testing.T) {
	t.Parallel()
	org, hc, addKey := e2eMtdMergeSetup(t)
	key := e2eMtdMergeKey()
	addKey(key)

	originalTags := []string{"e2e-prod", "e2e-team-a"}
	e2eMtdMergeSeed(t, hc, org.GetOID(), key,
		e2eMtdMergeData("indicator-original"),
		UsrMtd{Enabled: true, Expiry: 0, Tags: originalTags, Comment: "e2e-owned"},
	)

	src := fmt.Sprintf(`
hives:
  lookup:
    %s:
      data:
        lookup_data:
          indicator-updated: {}
`, key)
	e2eMtdMergePush(t, org, src)

	hd, err := hc.Get(HiveArgs{HiveName: e2eMtdMergeHive, PartitionKey: org.GetOID(), Key: key})
	require.NoError(t, err)
	assert.Contains(t, hd.Data["lookup_data"], "indicator-updated", "data should reflect the YAML value")
	assert.True(t, hd.UsrMtd.Enabled, "enabled must NOT be silently flipped when YAML omits usr_mtd")
	assert.Equal(t, int64(0), hd.UsrMtd.Expiry, "expiry must be preserved")
	assert.ElementsMatch(t, originalTags, hd.UsrMtd.Tags, "tags must be preserved")
	assert.Equal(t, "e2e-owned", hd.UsrMtd.Comment, "comment must be preserved")
}

// TestE2ESyncHive_PartialUsrMtd_PreservesUnauthored covers the subtle
// case: YAML carries usr_mtd with only a tags entry. The new tags must
// win on the wire, but enabled/expiry/comment must remain untouched.
func TestE2ESyncHive_PartialUsrMtd_PreservesUnauthored(t *testing.T) {
	t.Parallel()
	org, hc, addKey := e2eMtdMergeSetup(t)
	key := e2eMtdMergeKey()
	addKey(key)

	e2eMtdMergeSeed(t, hc, org.GetOID(), key,
		e2eMtdMergeData("indicator-stable"),
		UsrMtd{Enabled: true, Expiry: 0, Tags: []string{"old"}, Comment: "preserve-me"},
	)

	src := fmt.Sprintf(`
hives:
  lookup:
    %s:
      data:
        lookup_data:
          indicator-stable: {}
      usr_mtd:
        tags: ["new"]
`, key)
	e2eMtdMergePush(t, org, src)

	hd, err := hc.Get(HiveArgs{HiveName: e2eMtdMergeHive, PartitionKey: org.GetOID(), Key: key})
	require.NoError(t, err)
	assert.Equal(t, []string{"new"}, hd.UsrMtd.Tags, "tags must be the authored value")
	assert.True(t, hd.UsrMtd.Enabled, "enabled must be preserved when YAML did not author it")
	assert.Equal(t, "preserve-me", hd.UsrMtd.Comment, "comment must be preserved")
}

// TestE2ESyncHive_ExplicitEnabledFalse_Disables is the intent-honored
// case: when the YAML explicitly carries `enabled: false`, the rule must
// end up disabled. Distinguishes the merge from a naive "always
// preserve" that would block legitimate disables.
func TestE2ESyncHive_ExplicitEnabledFalse_Disables(t *testing.T) {
	t.Parallel()
	org, hc, addKey := e2eMtdMergeSetup(t)
	key := e2eMtdMergeKey()
	addKey(key)

	e2eMtdMergeSeed(t, hc, org.GetOID(), key,
		e2eMtdMergeData("indicator-x"),
		UsrMtd{Enabled: true, Tags: []string{"keep"}},
	)

	src := fmt.Sprintf(`
hives:
  lookup:
    %s:
      data:
        lookup_data:
          indicator-x: {}
      usr_mtd:
        enabled: false
`, key)
	e2eMtdMergePush(t, org, src)

	hd, err := hc.Get(HiveArgs{HiveName: e2eMtdMergeHive, PartitionKey: org.GetOID(), Key: key})
	require.NoError(t, err)
	assert.False(t, hd.UsrMtd.Enabled, "explicit enabled:false must be honored end-to-end")
	assert.ElementsMatch(t, []string{"keep"}, hd.UsrMtd.Tags, "tags not authored, must be preserved")
}

// TestE2ESyncHive_Add_DefaultsEnabledTrue exercises the create path:
// pushing YAML for a record that does not yet exist, with no enabled
// authored, must create the record enabled. Before the fix, the Add
// flow would carry the Go bool zero (`enabled: false`) to the server
// and the record would be created disabled.
func TestE2ESyncHive_Add_DefaultsEnabledTrue(t *testing.T) {
	t.Parallel()
	org, hc, addKey := e2eMtdMergeSetup(t)
	key := e2eMtdMergeKey()
	addKey(key)

	src := fmt.Sprintf(`
hives:
  lookup:
    %s:
      data:
        lookup_data:
          indicator-brand-new: {}
`, key)
	e2eMtdMergePush(t, org, src)

	hd, err := hc.Get(HiveArgs{HiveName: e2eMtdMergeHive, PartitionKey: org.GetOID(), Key: key})
	require.NoError(t, err)
	assert.True(t, hd.UsrMtd.Enabled, "new lookup record from YAML must default to enabled")
	assert.Contains(t, hd.Data["lookup_data"], "indicator-brand-new", "data field must reflect the YAML")
}

// TestE2ESyncHive_Add_ExplicitEnabledFalse_Respected is the create-path
// negative: when the YAML explicitly carries enabled:false on a new
// record, the new default does not override the explicit value.
func TestE2ESyncHive_Add_ExplicitEnabledFalse_Respected(t *testing.T) {
	t.Parallel()
	org, hc, addKey := e2eMtdMergeSetup(t)
	key := e2eMtdMergeKey()
	addKey(key)

	src := fmt.Sprintf(`
hives:
  lookup:
    %s:
      data:
        lookup_data:
          indicator-starts-disabled: {}
      usr_mtd:
        enabled: false
`, key)
	e2eMtdMergePush(t, org, src)

	hd, err := hc.Get(HiveArgs{HiveName: e2eMtdMergeHive, PartitionKey: org.GetOID(), Key: key})
	require.NoError(t, err)
	assert.False(t, hd.UsrMtd.Enabled, "explicit enabled:false on create must not be overridden by Add default")
}

// TestE2ESyncHive_AsSyncDataRoundTrip_NoChange validates the
// fetch-then-push workflow. A consumer doing sync fetch -> edit ->
// sync push of an unchanged YAML must not mutate any field of the
// record. The record is fetched, converted via AsSyncData, packaged
// into an OrgConfig that contains ONLY this record (so SyncPush does
// not touch any unrelated lookup records in the test org), and pushed
// back.
func TestE2ESyncHive_AsSyncDataRoundTrip_NoChange(t *testing.T) {
	t.Parallel()
	org, hc, addKey := e2eMtdMergeSetup(t)
	key := e2eMtdMergeKey()
	addKey(key)

	originalMtd := UsrMtd{
		Enabled: true,
		Expiry:  0,
		Tags:    []string{"e2e-roundtrip-a", "e2e-roundtrip-b"},
		Comment: "e2e-roundtrip",
	}
	e2eMtdMergeSeed(t, hc, org.GetOID(), key,
		e2eMtdMergeData("indicator-rt-original"),
		originalMtd,
	)

	// Fetch the single record we just created and build a config that
	// contains only it - we deliberately do not List the whole hive
	// because the test org may contain unrelated lookup records that we
	// must not touch.
	hd, err := hc.Get(HiveArgs{HiveName: e2eMtdMergeHive, PartitionKey: org.GetOID(), Key: key})
	require.NoError(t, err)

	cfg := OrgConfig{
		Hives: orgSyncHives{
			e2eMtdMergeHive: {
				key: hd.AsSyncData(),
			},
		},
	}
	_, err = org.SyncPush(cfg, SyncOptions{
		SyncHives: map[string]bool{e2eMtdMergeHive: true},
	})
	require.NoError(t, err)

	after, err := hc.Get(HiveArgs{HiveName: e2eMtdMergeHive, PartitionKey: org.GetOID(), Key: key})
	require.NoError(t, err)
	assert.Contains(t, after.Data["lookup_data"], "indicator-rt-original")
	assert.Equal(t, originalMtd.Enabled, after.UsrMtd.Enabled)
	assert.Equal(t, originalMtd.Expiry, after.UsrMtd.Expiry)
	assert.ElementsMatch(t, originalMtd.Tags, after.UsrMtd.Tags)
	assert.Equal(t, originalMtd.Comment, after.UsrMtd.Comment)
}
