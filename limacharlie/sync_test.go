package limacharlie

import (
	//"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
	"testing"
)

func TestSyncPush(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	rules, err := org.DRRules()
	a.NoError(err)
	if len(rules) != 0 {
		t.Errorf("unexpected preexisting rules in add/delete: %+v", rules)
	}

	yc := `
rules:
  r1:
    detect:
      op: is
      event: NEW_PROCESS
      path: event/FILE_PATH
      value: nope1
    respons:
      - action: report
        name: t1
  r2:
    detect:
      op: is
      event: NEW_PROCESS
      path: event/FILE_PATH
      value: nope2
    respons:
      - action: report
        name: t2
  r3:
    namespace: managed
    detect:
      op: is
      event: NEW_PROCESS
      path: event/FILE_PATH
      value: nope3
    respons:
      - action: report
        name: t3
`
	c := OrgConfig{}
	if err := yaml.Unmarshal([]byte(yc), &c); err != nil {
		t.Errorf("yaml.Unmarshal: %v", err)
	}

	if len(c.DRRules) != 3 {
		t.Errorf("unexpected conf: %+v", c)
	}

	ops, err := org.SyncPush(c, SyncOptions{
		IsDryRun:    true,
		SyncDRRules: true,
	})
	a.NoError(err)

	if len(ops) != 3 {
		t.Errorf("unexpected ops: %+v", err)
	}
}
