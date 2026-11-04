package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"ds2api/internal/config"
)

type DeviceHarvester interface {
	Enabled() bool
	Harvest(context.Context, string) (string, error)
}

type deviceHarvestCall struct {
	done  chan struct{}
	token string
	err   error
}

type deviceRejection interface {
	DeviceRejected() bool
}

func (r *Resolver) harvestDevice(ctx context.Context, accountID string) (string, error) {
	if accountID == "" {
		return "", errors.New("cannot harvest a device token without an account identifier")
	}
	r.harvestMu.Lock()
	if r.harvestCalls == nil {
		r.harvestCalls = make(map[string]*deviceHarvestCall)
	}
	if call := r.harvestCalls[accountID]; call != nil {
		r.harvestMu.Unlock()
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-call.done:
			return call.token, call.err
		}
	}
	call := &deviceHarvestCall{done: make(chan struct{})}
	r.harvestCalls[accountID] = call
	r.harvestMu.Unlock()

	token, err := r.DeviceHarvest.Harvest(ctx, accountID)
	token = strings.TrimSpace(token)
	if err == nil && token == "" {
		err = errors.New("device harvest returned an empty token")
	}
	r.harvestMu.Lock()
	call.token, call.err = token, err
	delete(r.harvestCalls, accountID)
	close(call.done)
	r.harvestMu.Unlock()
	return token, err
}

// loginWithDevice uses an existing token, or optionally obtains one from the
// configured sidecar. A rejected token is replaced once, with no unbounded retry.
func (r *Resolver) loginWithDevice(ctx context.Context, acc config.Account) (string, string, error) {
	harvestEnabled := r.DeviceHarvest != nil && r.DeviceHarvest.Enabled()
	if strings.TrimSpace(acc.DeviceID) == "" && harvestEnabled {
		deviceID, err := r.harvestDevice(ctx, acc.Identifier())
		if err != nil {
			return "", "", fmt.Errorf("harvest device token: %w", err)
		}
		acc.DeviceID = deviceID
	}
	token, err := r.Login(ctx, acc)
	var rejected deviceRejection
	if err == nil || !harvestEnabled || !errors.As(err, &rejected) || !rejected.DeviceRejected() {
		return token, acc.DeviceID, err
	}
	deviceID, harvestErr := r.harvestDevice(ctx, acc.Identifier())
	if harvestErr != nil {
		return "", acc.DeviceID, fmt.Errorf("refresh device token: %w", harvestErr)
	}
	acc.DeviceID = deviceID
	token, err = r.Login(ctx, acc)
	return token, acc.DeviceID, err
}
