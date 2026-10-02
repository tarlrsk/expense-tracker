package create

import "example.com/fx/internal/tx"

// Violation: a categories processor holds the auth transactor.
type Processor struct{ auth tx.Auth }
