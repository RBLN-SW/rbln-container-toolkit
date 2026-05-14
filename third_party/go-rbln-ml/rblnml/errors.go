package rblnml

import "fmt"

// ReturnCode mirrors rblnmlReturn_t from rblnml.h.
type ReturnCode uint32

const (
	Success             ReturnCode = 0
	ErrorUninitialized  ReturnCode = 1
	ErrorInvalidArg     ReturnCode = 2
	ErrorNoPermission   ReturnCode = 3
	ErrorNotFound       ReturnCode = 4
	ErrorIoctlFailed    ReturnCode = 5
	ErrorUnknown        ReturnCode = 999
)

func (rc ReturnCode) String() string {
	switch rc {
	case Success:
		return "RBLNML_SUCCESS"
	case ErrorUninitialized:
		return "RBLNML_ERROR_UNINITIALIZED"
	case ErrorInvalidArg:
		return "RBLNML_ERROR_INVALID_ARGUMENT"
	case ErrorNoPermission:
		return "RBLNML_ERROR_NO_PERMISSION"
	case ErrorNotFound:
		return "RBLNML_ERROR_NOT_FOUND"
	case ErrorIoctlFailed:
		return "RBLNML_ERROR_IOCTL_FAILED"
	case ErrorUnknown:
		return "RBLNML_ERROR_UNKNOWN"
	default:
		return fmt.Sprintf("RBLNML_ERROR_UNKNOWN(%d)", uint32(rc))
	}
}

// RblnmlError wraps a non-success ReturnCode as a Go error.
type RblnmlError struct {
	Code ReturnCode
}

func (e *RblnmlError) Error() string {
	return fmt.Sprintf("rblnml error: %s", e.Code)
}

// toError converts a ReturnCode to nil (on success) or *RblnmlError.
func toError(rc ReturnCode) error {
	if rc == Success {
		return nil
	}
	return &RblnmlError{Code: rc}
}
