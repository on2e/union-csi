package union

import (
	"errors"
)

// Union errors that can relate to gRPC error codes, first iteration
var (
	ErrIdempotencyIncompatible = errors.New("idempotent request is incompatible with previous request(s)")
	ErrVolumeNotFound          = errors.New("volume resource is not found")
	ErrNodeNotFound            = errors.New("node resource is not found")
	ErrAttachmentNotFound      = errors.New("attachment resource is not found")
	ErrVolumeInUse             = errors.New("volume resource is in use")
)
