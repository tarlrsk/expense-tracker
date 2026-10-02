package app

import "example.com/fx/internal/db"

// Violation: app may import db, but not use the auth connection itself.
func Run() { _, _ = db.AuthConn(nil) }
