package db

func AuthConn(any) (any, error) { return nil, nil }

// Allowed: db itself.
func use() { _, _ = AuthConn(nil) }
