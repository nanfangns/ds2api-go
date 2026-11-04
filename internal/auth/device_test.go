package auth

import (
	"context"
	"errors"
	"testing"

	"ds2api/internal/config"
)

type testDeviceHarvester struct{ calls int }

func (*testDeviceHarvester) Enabled() bool { return true }
func (h *testDeviceHarvester) Harvest(context.Context, string) (string, error) {
	h.calls++
	return "B-fresh", nil
}

type testDeviceRejection struct{}

func (testDeviceRejection) Error() string        { return "device rejected" }
func (testDeviceRejection) DeviceRejected() bool { return true }

func TestLoginWithDeviceHarvestsAndRetriesOnlyDeviceRejection(t *testing.T) {
	harvester := &testDeviceHarvester{}
	var devices []string
	r := &Resolver{DeviceHarvest: harvester, Login: func(_ context.Context, acc config.Account) (string, error) {
		devices = append(devices, acc.DeviceID)
		if len(devices) == 1 {
			return "", testDeviceRejection{}
		}
		return "login-token", nil
	}}
	token, device, err := r.loginWithDevice(context.Background(), config.Account{Email: "u@example.com", DeviceID: "B-old"})
	if err != nil || token != "login-token" || device != "B-fresh" || harvester.calls != 1 || len(devices) != 2 {
		t.Fatalf("unexpected login result: token=%q device=%q calls=%d devices=%v err=%v", token, device, harvester.calls, devices, err)
	}

	harvester.calls = 0
	r.Login = func(context.Context, config.Account) (string, error) { return "", errors.New("incorrect password") }
	_, _, err = r.loginWithDevice(context.Background(), config.Account{Email: "u@example.com", DeviceID: "B-old"})
	if err == nil || harvester.calls != 0 {
		t.Fatalf("non-device failure should not trigger harvest, calls=%d err=%v", harvester.calls, err)
	}
}

func TestLoginWithDeviceHarvestsWhenMissing(t *testing.T) {
	harvester := &testDeviceHarvester{}
	r := &Resolver{DeviceHarvest: harvester, Login: func(_ context.Context, acc config.Account) (string, error) {
		if acc.DeviceID != "B-fresh" {
			t.Fatalf("missing harvested token")
		}
		return "login-token", nil
	}}
	_, device, err := r.loginWithDevice(context.Background(), config.Account{Email: "u@example.com"})
	if err != nil || device != "B-fresh" || harvester.calls != 1 {
		t.Fatalf("unexpected result: device=%q calls=%d err=%v", device, harvester.calls, err)
	}
}
