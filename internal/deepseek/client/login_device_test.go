package client

import (
	"testing"

	"ds2api/internal/config"
)

func TestLoginPayloadUsesConfiguredDeviceID(t *testing.T) {
	payload, err := loginPayload(config.Account{Email: "a@example.com", Password: "pw", DeviceID: "  B-device  "})
	if err != nil || payload["device_id"] != "B-device" {
		t.Fatalf("expected configured device id, got payload=%v err=%v", payload, err)
	}
	legacy, err := loginPayload(config.Account{Email: "a@example.com", Password: "pw"})
	if err != nil || legacy["device_id"] != "deepseek_to_api" {
		t.Fatalf("expected legacy fallback, got payload=%v err=%v", legacy, err)
	}
}
