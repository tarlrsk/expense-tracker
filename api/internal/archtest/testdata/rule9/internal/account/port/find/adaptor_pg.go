package find

import "example.com/fx/internal/db"

// Allowed: an account adaptor uses the auth connection.
func Find() { _, _ = db.AuthConn(nil) }
