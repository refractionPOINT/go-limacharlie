# go-limacharlie

Go SDK for LimaCharlie. `README.md` covers the layout and how to run the tests.
`CHANGELOG.md` records every user-visible change under "Unreleased".

## Adding a hive to sync, or anything else that needs a new permission

The sync code (`limacharlie/sync.go`, `limacharlie/sync_hive.go`) runs behind
the Infrastructure extension and Git Sync. Customers use it to back up and
restore whole orgs, often from a scheduled job using an API key per org that
holds only the permissions it needed when it was made. A hive added to
`KnownHives` or `SyncAll()` is read by every "sync everything" run once the
extension picks up this version. A key that lacks the hive's read permission
then fails the whole org's sync, unless the caller set
`SyncOptions.SkipUnauthorizedHives`. Customers can only find out which
permission to grant from the release notes, so in the same change:

1. **Add a "New permissions" entry to `CHANGELOG.md` under "Unreleased".** Name
   the hive and every permission it takes: the read permission `SyncFetch`
   needs, and the write permissions `SyncPush` needs. Say plainly that a full
   sync fails with a key that lacks the read permission unless unreadable hives
   are skipped. Example:

   ```markdown
   ### New permissions: `host_posture_rule` hive

   - `KnownHives` and `SyncAll()` now include `host_posture_rule`. Reading it
     needs `host_posture.get`; writing rules needs `host_posture.set` and
     `sensor.task`. A full sync with a key without `host_posture.get` fails
     unless `SkipUnauthorizedHives` is set.
   ```

2. **Take the permission names from the hive's definition in the Hive
   service, not from the hive name.** They often differ: `host_posture_rule`
   reads with `host_posture.get`, and `external_adapter` with
   `externaladapter.get.mtd`. Never guess a name. If you can't confirm it, say
   so in the PR and ask.
3. **Do the same for any new SDK call that needs a permission no earlier
   version needed.** The same rule applies: one changelog line naming the
   permission.
4. **Add the hive to the Python CLI too** (`_HIVE_FLAG_MAP` in
   python-limacharlie's `limacharlie/commands/sync.py`). Its `sync pull --all`
   uses that list, and that repo has the same rule.
