package login

import "example.com/fx/internal/tx"

// Allowed: the account module uses the auth transactor.
type Processor struct{ auth tx.Auth }

func (p Processor) Execute() { _ = p.auth.WithAuthTx(nil, nil) }
