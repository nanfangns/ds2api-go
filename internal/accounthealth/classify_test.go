package accounthealth

import (
	"testing"
	"time"
)

func TestClassifyExplicitRestrictions(t *testing.T) {
	now := time.Unix(1700000000, 0)
	cases := []struct {
		body, status string
		until        int64
	}{
		{`{"data":{"biz_code":5,"biz_msg":"user is muted","biz_data":{"is_muted":1,"mute_until":1700003600}}}`, "muted", 1700003600},
		{`{"data":{"biz_msg":"user is muted"}}`, "muted", 1700003600},
		{`{"data":{"biz_msg":"account has been banned"}}`, "banned", 0},
		{`{"data":{"biz_code":11,"biz_msg":"RISK_DEVICE_DETECTED"}}`, "", 0},
		{`{"data":{"biz_msg":"forbidden"}}`, "", 0},
		{`{"data":{"biz_data":{"is_muted":0}}}`, "", 0},
		{`{"message":"account has been banned"}`, "", 0},
		{`connection reset by peer`, "", 0},
	}
	for _, c := range cases {
		t.Run(c.body, func(t *testing.T) {
			status, until := Classify([]byte(c.body), now)
			if status != c.status || until != c.until {
				t.Fatalf("got %s/%d, want %s/%d", status, until, c.status, c.until)
			}
		})
	}
}
