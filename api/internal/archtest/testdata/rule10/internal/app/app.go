package app

import "example.com/fx/internal/tx"

// Allowed: app hands the auth transactor to the account module.
var auth tx.Auth

func Run() { _ = auth.WithAuthTx(nil, nil) }
