package txtest

import "example.com/fx/internal/tx"

// Allowed: the fake implements the auth transactor.
var _ tx.Auth = (*Fake)(nil)

type Fake struct{}

func (f *Fake) WithAuthTx(any, any) error { return nil }
