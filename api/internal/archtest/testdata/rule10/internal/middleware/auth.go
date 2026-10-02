package middleware

type opener interface{ Open() error }

// Violation: a call named WithAuthTx, whatever the receiver.
func Session(d interface{ opener }) { _ = d.(interface{ WithAuthTx() error }) }
