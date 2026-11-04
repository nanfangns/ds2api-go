package auth

import (
	"time"

	"ds2api/internal/accounthealth"
	"ds2api/internal/config"
)

// RecordAccountFailure quarantines only explicitly restricted managed accounts.
func (r *Resolver) RecordAccountFailure(a *RequestAuth, raw []byte) bool {
	if r == nil || r.Store == nil || r.Pool == nil || a == nil || !a.UseConfigToken || a.AccountID == "" {
		return false
	}
	status, until := accounthealth.Classify(raw, time.Now())
	if status == "" {
		return false
	}
	if err := r.Store.UpdateAccountPoolStatus(a.AccountID, status, until); err != nil {
		config.Logger.Warn("[account_health] failed to save account status", "account", a.AccountID, "error", err)
	}
	r.Pool.Reset()
	return true
}

func (a *RequestAuth) RecordAccountFailure(raw []byte) bool {
	return a != nil && a.resolver != nil && a.resolver.RecordAccountFailure(a, raw)
}
