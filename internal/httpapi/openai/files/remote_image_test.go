package files

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"testing"
)

type imageRoundTripper func(*http.Request) (*http.Response, error)

func (f imageRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRemoteImageRejectsNonPublicAddresses(t *testing.T) {
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "100.64.0.1", "::1", "fc00::1", "::ffff:127.0.0.1"} {
		if validPublicImageAddress(netip.MustParseAddr(raw)) {
			t.Errorf("accepted non-public address %s", raw)
		}
	}
	if !validPublicImageAddress(netip.MustParseAddr("8.8.8.8")) {
		t.Fatal("rejected a public address")
	}
	if _, err := dialPublicImage(context.Background(), "tcp", "127.0.0.1:80"); err == nil {
		t.Fatal("dial to loopback must be rejected")
	}
}

func TestRemoteImageValidatesBodyAndSize(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\ncontent")
	client := &http.Client{Transport: imageRoundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(png)), Header: make(http.Header)}, nil
	})}
	data, contentType, err := fetchRemoteImageWithClient(context.Background(), "https://example.com/image.png", client)
	if err != nil || !bytes.Equal(data, png) || contentType != "image/png" {
		t.Fatalf("unexpected image result: type=%q err=%v", contentType, err)
	}
	for _, raw := range []string{"http://user:pw@example.com/a", "http://example.com:8080/a", "file:///etc/passwd"} {
		if _, _, err := fetchRemoteImageWithClient(context.Background(), raw, client); err == nil {
			t.Errorf("accepted invalid URL %q", raw)
		}
	}
	client.Transport = imageRoundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("not an image")), Header: make(http.Header)}, nil
	})
	if _, _, err := fetchRemoteImageWithClient(context.Background(), "https://example.com/a", client); err == nil {
		t.Fatal("accepted non-image body")
	}
}

func TestDecodeRemoteAndCompactImageInputs(t *testing.T) {
	remote, ok, err := decodeOpenAIInlineFileBlock(map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/a.png"}})
	if err != nil || !ok || remote.RemoteURL != "https://example.com/a.png" || remote.ReplacementType != "input_image" {
		t.Fatalf("unexpected remote decode: %+v ok=%v err=%v", remote, ok, err)
	}
	inline, ok, err := decodeOpenAIInlineFileBlock(map[string]any{"type": "image", "data": "aGVsbG8=", "mime_type": "image/png"})
	if err != nil || !ok || string(inline.Data) != "hello" || inline.ReplacementType != "input_image" {
		t.Fatalf("unexpected inline decode: %+v ok=%v err=%v", inline, ok, err)
	}
}
