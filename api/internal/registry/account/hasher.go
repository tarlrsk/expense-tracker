package account

import (
	"sync"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
)

// hasher is the password hasher shared by the account use cases. Its dummy hash (for logins
// without a real one) is made once, when the first use case is built at startup (ADR-0066).
var hasher = sync.OnceValue(func() *domain.Hasher { return domain.NewHasher(domain.DefaultHashParams) })
