package tenant

// UserRepository defines the interface for user data persistence.
type UserRepository interface {
	Create(u *User) error
	GetByID(id string) (*User, error)
	GetByEmail(tenantID, email string) (*User, error)
	GetByEmailGlobal(email string) ([]*User, error)
	Update(u *User) error
	Delete(id string) error
}
