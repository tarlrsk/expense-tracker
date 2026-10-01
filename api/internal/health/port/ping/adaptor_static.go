package ping

import "context"

type static struct{}

// NewStatic returns a placeholder adaptor that always answers ok.
func NewStatic() Port { return static{} }

func (static) Ping(context.Context) error { return nil }
