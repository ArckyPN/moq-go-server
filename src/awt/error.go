package awt

import "errors"

var (
	ErrInvalidOS error = errors.New("invalid OS")

	ErrInvalidConnectionTracingKey error = errors.New("failed to convert ConnectionTracingKey")
)
