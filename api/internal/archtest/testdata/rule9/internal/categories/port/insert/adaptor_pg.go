package insert

import "example.com/fx/internal/db"

// Violation: a categories adaptor uses the auth connection.
func Insert() { _, _ = db.AuthConn(nil) }
