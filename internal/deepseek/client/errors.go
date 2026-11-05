package client

import (
	"errors"
	"fmt"
)

type FailureKind string

const (
	FailureUnknown             FailureKind = ""
	FailureDirectUnauthorized  FailureKind = "direct_unauthorized"
	FailureManagedUnauthorized FailureKind = "managed_unauthorized"
	FailureDeviceRejected      FailureKind = "device_rejected"
	FailureAccountRestricted   FailureKind = "account_restricted"
)

type RequestFailure struct {
	Op      string
	Kind    FailureKind
	Message string
}

func (e *RequestFailure) DeviceRejected() bool {
	return e != nil && e.Kind == FailureDeviceRejected
}

func (e *RequestFailure) Error() string {
	if e == nil {
		return ""
	}
	switch {
	case e.Op != "" && e.Message != "":
		return fmt.Sprintf("%s: %s", e.Op, e.Message)
	case e.Op != "":
		return e.Op + " failed"
	case e.Message != "":
		return e.Message
	default:
		return "request failed"
	}
}

func IsManagedUnauthorizedError(err error) bool {
	var failure *RequestFailure
	return errors.As(err, &failure) && failure.Kind == FailureManagedUnauthorized
}

func IsDirectUnauthorizedError(err error) bool {
	var failure *RequestFailure
	return errors.As(err, &failure) && failure.Kind == FailureDirectUnauthorized
}

func IsAccountRestrictedError(err error) bool {
	var failure *RequestFailure
	return errors.As(err, &failure) && failure.Kind == FailureAccountRestricted
}
