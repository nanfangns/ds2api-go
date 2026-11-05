package client

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"ds2api/internal/account"
	"ds2api/internal/auth"
	"ds2api/internal/config"
)

func TestSessionAndPowKeepAccountAffinity(t *testing.T) {
	for _, op := range []string{"session", "pow"} {
		for _, restricted := range []bool{false, true} {
			t.Run(op+map[bool]string{false: "-transient", true: "-restricted"}[restricted], func(t *testing.T) {
				t.Setenv("DS2API_CONFIG_JSON", `{"keys":["key"],"accounts":[{"email":"first@example.test","password":"pwd"},{"email":"second@example.test","password":"pwd"}]}`)
				store := config.LoadStore()
				resolver := auth.NewResolver(store, account.NewPool(store), func(_ context.Context, acc config.Account) (string, error) { return "token-" + acc.Identifier(), nil })
				req, _ := http.NewRequest(http.MethodPost, "/", nil)
				req.Header.Set("Authorization", "Bearer key")
				a, err := resolver.Determine(req)
				if err != nil {
					t.Fatal(err)
				}
				defer resolver.Release(a)
				calls := 0
				c := &Client{Auth: resolver, Store: store, maxRetries: 3, regular: doerFunc(func(r *http.Request) (*http.Response, error) {
					calls++
					if r.Header.Get("Authorization") != "Bearer token-first@example.test" {
						t.Fatalf("request changed account: %q", r.Header.Get("Authorization"))
					}
					body := `{"code":1,"msg":"temporary problem"}`
					if restricted {
						body = `{"code":0,"data":{"biz_code":14,"biz_msg":"user is muted"}}`
					}
					return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
				})}
				if op == "session" {
					_, err = c.CreateSession(context.Background(), a, 3)
				} else {
					_, err = c.GetPow(context.Background(), a, 3)
				}
				if err == nil || a.AccountID != "first@example.test" {
					t.Fatalf("expected failure without switching account: account=%s err=%v", a.AccountID, err)
				}
				if restricted && (!IsAccountRestrictedError(err) || calls != 1) {
					t.Fatalf("expected one restricted attempt: calls=%d err=%v", calls, err)
				}
				if !restricted && calls != 3 {
					t.Fatalf("expected three same-account retries, got %d", calls)
				}
			})
		}
	}
}
