package client

import (
	"context"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"testing"

	"ds2api/internal/account"
	"ds2api/internal/auth"
	"ds2api/internal/config"
	dsprotocol "ds2api/internal/deepseek/protocol"
	powpkg "ds2api/pow"
)

func TestUploadFileRecognizesAndQuarantinesMutedAccount(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{"keys":["key"],"accounts":[{"email":"muted@example.test","password":"pwd"}]}`)
	store := config.LoadStore()
	resolver := auth.NewResolver(store, account.NewPool(store), func(_ context.Context, _ config.Account) (string, error) { return "test-token", nil })
	request, _ := http.NewRequest(http.MethodPost, "/", nil)
	request.Header.Set("Authorization", "Bearer key")
	a, err := resolver.Determine(request)
	if err != nil {
		t.Fatal(err)
	}
	defer resolver.Release(a)
	challengeHash := powpkg.DeepSeekHashV1([]byte(powpkg.BuildPrefix("salt", 1712345678) + "42"))
	powResponse := `{"code":0,"data":{"biz_code":0,"biz_data":{"challenge":{"algorithm":"DeepSeekHashV1","challenge":"` + hex.EncodeToString(challengeHash[:]) + `","salt":"salt","expire_at":1712345678,"difficulty":1000,"signature":"sig","target_path":"` + dsprotocol.DeepSeekUploadTargetPath + `"}}}}`
	calls := 0
	c := &Client{Auth: resolver, Store: store, maxRetries: 3, regular: doerFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		body := powResponse
		if calls == 2 {
			body = `{"code":0,"data":{"biz_code":14,"biz_msg":"user is muted"}}`
		}
		if calls > 2 {
			t.Fatal("restricted account was retried")
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	})}
	_, err = c.UploadFile(context.Background(), a, UploadFileRequest{Filename: "sample.txt", Data: []byte("hello")}, 3)
	if !IsAccountRestrictedError(err) {
		t.Fatalf("expected account restriction, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected one pow and one upload, got %d calls", calls)
	}
	acc, ok := store.FindAccount("muted@example.test")
	if !ok || acc.PoolStatus != "muted" || acc.MuteUntil == 0 {
		t.Fatalf("account restriction not recorded: %+v", acc)
	}
	if _, ok := resolver.Pool.Acquire("muted@example.test", nil); ok {
		t.Fatal("restricted account still eligible in this instance")
	}
}
