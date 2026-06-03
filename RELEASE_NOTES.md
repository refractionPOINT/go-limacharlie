# Release Notes

## Unreleased

- Fix `HiveClient.Update` wholesale-replacing `usr_mtd` with Go zero values for fields the caller did not author.

  Before this change, the hive sync push flow rebuilt `usr_mtd` from the caller's args alone and sent a fully-populated block on every write. Combined with the server's wholesale-replace semantic on `set_record` and `set_record_mtd`, any field the caller did not explicitly author landed on the wire as a zero value (`enabled: false`, `expiry: 0`, `tags: nil`, `comment: ""`) and overwrote whatever was on the record.

  The most visible consequence was silent disable: YAML that omitted `usr_mtd:` (or only partially populated it) for an existing rule would, on the next sync push, flip the rule to disabled and strip its tags/comment/expiry, without any error or warning.

  `HiveClient.Update` now fetches the existing record (which it already did for etag handling) and uses its `usr_mtd` as the merge base. Only fields the caller explicitly set via non-nil `Enabled`/`Expiry`/`Comment` pointers or non-nil `Tags` slice overlay the existing values; everything else is preserved. The server's existing etag CAS continues to gate the write, so a concurrent writer between the SDK's fetch and the SDK's POST surfaces as `ETAG_MISMATCH` instead of a silent stomp.

  The sync push flow (`SyncHiveData` + `syncHive`) now tracks which `usr_mtd` keys were authored in the source YAML via a custom `UnmarshalYAML`, and forwards only authored fields to `HiveClient.Update`. Sparse YAML stops mutating fields the user did not author. `Equals` is presence-aware so sparse YAML matching current state no longer triggers spurious no-op `UPDATE` calls.

  `HiveClient.Add` defaults `Enabled: true` for YAML-loaded new records when the YAML did not specify enabled, since declaring a rule via IaC almost always implies the author wants it active. Direct callers passing explicit pointer args are unaffected.

  Tag semantic is now explicit at the SDK boundary: `Tags: nil` preserves existing tags, `Tags: []string{}` (non-nil empty slice) clears them.

  Behavior change to be aware of for direct `HiveClient.Update` callers: a caller passing a non-nil pointer with an explicit zero value (e.g. `Enabled: &false`) still gets that zero applied. A caller passing nil pointers - which previously silently wiped fields to Go zero values - now preserves the existing field. This is strictly the friendlier semantic and matches what such callers almost certainly intended. No public type signatures changed.

  See [#243](https://github.com/refractionPOINT/go-limacharlie/pull/243) for details, including the related wire-format-ambiguity issues elsewhere in the sync flow that are scoped for follow-up.
