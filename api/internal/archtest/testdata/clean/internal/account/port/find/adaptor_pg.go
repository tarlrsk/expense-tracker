package find

import "example.com/fx/internal/db"

func find() { _, _ = db.AuthConn(nil) }
