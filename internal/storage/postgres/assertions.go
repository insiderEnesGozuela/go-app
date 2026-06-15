package postgres

import "github.com/insiderEnesGozuela/go-app/internal/repository"

// Compile-time guarantees that the Postgres types satisfy the repository
// contracts the service layer depends on. If a method signature ever drifts
// from the interface, the build breaks here — at the seam — instead of at some
// distant call site. The `var _ Iface = (*T)(nil)` idiom costs nothing at
// runtime (it's a nil pointer in a discarded variable).
var (
	_ repository.UserRepository        = (*UserRepository)(nil)
	_ repository.WalletRepository      = (*WalletRepository)(nil)
	_ repository.TransactionRepository = (*TransactionRepository)(nil)
	_ repository.UnitOfWork            = (*UnitOfWork)(nil)
)
