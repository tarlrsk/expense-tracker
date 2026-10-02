package account

import "example.com/fx/internal/tx"

func Register(auth tx.Auth) { _ = auth.WithAuthTx(nil, nil) }
