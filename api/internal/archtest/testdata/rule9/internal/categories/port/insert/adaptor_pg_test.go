package insert

import pg "example.com/fx/internal/db"

// Violation: an aliased import does not hide it.
func helper() { _, _ = pg.AuthConn(nil) }
