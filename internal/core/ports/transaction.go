package ports

import "context"

// TransactionManager defines the unit-of-work boundary used by modules.
type TransactionManager interface {
	WithinTransaction(ctx context.Context, fn func(context.Context) error) error
}
