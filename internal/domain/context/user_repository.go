package context

// UserRepository defines the interface for user data persistence.
type UserRepository interface {
	Create(u *User) error
	GetByID(id string) (*User, error)
	GetByEmail(contextID, email string) (*User, error)
	GetByEmailGlobal(email string) ([]*User, error)
	GetByUsername(email string) (*User, error)         // platform-wide, users table
	ExistsAnyUser() (bool, error)                       // for bootstrap check
	ListAll() ([]*User, error)                          // list all platform users
	Update(u *User) error
	Delete(id string) error

	// Context membership operations
	GetByUserContext(userID, contextID string) (*UserContext, error)
	ListByUser(userID string) ([]UserContext, error)
	CreateMembership(uc *UserContext) error
	UpdateMembership(uc *UserContext) error
	DeleteMembership(userID, contextID string) error
	ListMembers(contextID string) ([]UserContext, error)
}
