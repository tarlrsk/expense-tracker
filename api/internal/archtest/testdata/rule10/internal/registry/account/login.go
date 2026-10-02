package account

import "example.com/fx/internal/tx"

// Allowed: the account registration receives the auth transactor.
func NewLogin(auth tx.Auth) {}
