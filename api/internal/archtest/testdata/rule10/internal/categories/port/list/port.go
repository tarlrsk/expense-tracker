package list

import "context"

// Violation: a local interface with a WithAuthTx method.
type authOpener interface {
	WithAuthTx(ctx context.Context, fn func(context.Context) error) error
}
