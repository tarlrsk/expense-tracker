// Package ping is the health module's liveness port.
package ping

import "context"

// Port answers whether the API can serve requests. It never touches the database.
type Port interface {
	Ping(ctx context.Context) error
}
