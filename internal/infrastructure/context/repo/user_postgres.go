package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	domainctx "github.com/hekemen/automata/internal/domain/context"
	"github.com/jackc/pgx/v5/pgxpool"
)

type userPostgresRepo struct {
	pool *pgxpool.Pool
}

// NewUserPostgresRepo creates a new PostgreSQL user repository.
func NewUserPostgresRepo(pool *pgxpool.Pool) domainctx.UserRepository {
	return &userPostgresRepo{pool: pool}
}

func (r *userPostgresRepo) Create(u *domainctx.User) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	if err := u.Validate(); err != nil {
		return err
	}

	// Insert into users table (platform-level)
	query := `
		INSERT INTO users (id, email, password_hash, sso_provider, sso_id, is_admin, display_name, avatar_url, password_changed_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW(), NOW())
		RETURNING id, email, password_hash, sso_provider, sso_id, is_admin, display_name, avatar_url, password_changed_at, created_at, updated_at
	`

	err := r.pool.QueryRow(context.Background(), query,
		u.ID, u.Email, u.PasswordHash, nil, nil, u.IsAdmin,
		u.DisplayName, u.AvatarURL,
	).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.SSOProvider, &u.SSOID,
		&u.IsAdmin, &u.DisplayName, &u.AvatarURL, &u.PasswordChangedAt,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *userPostgresRepo) GetByID(id string) (*domainctx.User, error) {
	u := &domainctx.User{}

	query := `
		SELECT id, email, password_hash, sso_provider, sso_id, is_admin, display_name, avatar_url, password_changed_at, created_at, updated_at
		FROM users WHERE id = $1
	`

	err := r.pool.QueryRow(context.Background(), query, id).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.SSOProvider, &u.SSOID,
		&u.IsAdmin, &u.DisplayName, &u.AvatarURL, &u.PasswordChangedAt,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get user by ID: %w", err)
	}

	return u, nil
}

func (r *userPostgresRepo) GetByEmail(contextID, email string) (*domainctx.User, error) {
	u := &domainctx.User{}

	query := `
		SELECT id, context_id, email, password_hash, is_owner, created_at, updated_at
		FROM context_users WHERE context_id = $1 AND email = $2
	`

	err := r.pool.QueryRow(context.Background(), query, contextID, email).Scan(
		&u.ID, &u.ContextID, &u.Email, &u.PasswordHash, &u.IsOwner,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}

	return u, nil
}

func (r *userPostgresRepo) GetByEmailGlobal(email string) ([]*domainctx.User, error) {
	query := `
		SELECT id, context_id, email, password_hash, is_owner, created_at, updated_at
		FROM context_users WHERE email = $1
	`

	rows, err := r.pool.Query(context.Background(), query, email)
	if err != nil {
		return nil, fmt.Errorf("get user by email (global): %w", err)
	}
	defer rows.Close()

	var users []*domainctx.User
	for rows.Next() {
		u := &domainctx.User{}
		err := rows.Scan(
			&u.ID, &u.ContextID, &u.Email, &u.PasswordHash, &u.IsOwner,
			&u.CreatedAt, &u.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}

	if len(users) == 0 {
		return nil, fmt.Errorf("user not found")
	}

	return users, nil
}

func (r *userPostgresRepo) GetByUsername(email string) (*domainctx.User, error) {
	u := &domainctx.User{}

	query := `
		SELECT id, email, password_hash, sso_provider, sso_id, is_admin, display_name, avatar_url, password_changed_at, created_at, updated_at
		FROM users WHERE email = $1
	`

	err := r.pool.QueryRow(context.Background(), query, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.SSOProvider, &u.SSOID,
		&u.IsAdmin, &u.DisplayName, &u.AvatarURL, &u.PasswordChangedAt,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get user by username: %w", err)
	}

	return u, nil
}

func (r *userPostgresRepo) ExistsAnyUser() (bool, error) {
	var exists bool
	err := r.pool.QueryRow(context.Background(),
		"SELECT EXISTS(SELECT 1 FROM users LIMIT 1)").Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check users existence: %w", err)
	}
	return exists, nil
}

func (r *userPostgresRepo) ListAll() ([]*domainctx.User, error) {
	rows, err := r.pool.Query(context.Background(), `
		SELECT id, email, password_hash, sso_provider, sso_id, is_admin, display_name, avatar_url, password_changed_at, created_at, updated_at
		FROM users
	`)
	if err != nil {
		return nil, fmt.Errorf("list all users: %w", err)
	}
	defer rows.Close()

	var result []*domainctx.User
	for rows.Next() {
		u := &domainctx.User{}
		if err := rows.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.SSOProvider,
			&u.SSOID, &u.IsAdmin, &u.DisplayName, &u.AvatarURL, &u.PasswordChangedAt,
			&u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		result = append(result, u)
	}
	return result, nil
}

func (r *userPostgresRepo) Update(u *domainctx.User) error {
	query := `
		UPDATE users SET email = $1, password_hash = $2, is_admin = $3, display_name = $4, avatar_url = $5, updated_at = NOW()
		WHERE id = $6
	`

	result, err := r.pool.Exec(context.Background(), query,
		u.Email, u.PasswordHash, u.IsAdmin, u.DisplayName, u.AvatarURL, u.ID,
	)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found: %s", u.ID)
	}

	return nil
}

func (r *userPostgresRepo) Delete(id string) error {
	query := `DELETE FROM users WHERE id = $1`
	result, err := r.pool.Exec(context.Background(), query, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found: %s", id)
	}

	return nil
}

func (r *userPostgresRepo) GetByUserContext(userID, contextID string) (*domainctx.UserContext, error) {
	uc := &domainctx.UserContext{}
	err := r.pool.QueryRow(context.Background(), `
		SELECT id, user_id, context_id, role, created_at, updated_at
		FROM user_contexts WHERE user_id = $1 AND context_id = $2
	`, userID, contextID).Scan(
		&uc.ID, &uc.UserID, &uc.ContextID, &uc.Role,
		&uc.CreatedAt, &uc.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get user context: %w", err)
	}
	return uc, nil
}

func (r *userPostgresRepo) ListByUser(userID string) ([]domainctx.UserContext, error) {
	rows, err := r.pool.Query(context.Background(), `
		SELECT id, user_id, context_id, role, created_at, updated_at
		FROM user_contexts WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user contexts: %w", err)
	}
	defer rows.Close()

	var result []domainctx.UserContext
	for rows.Next() {
		var uc domainctx.UserContext
		if err := rows.Scan(&uc.ID, &uc.UserID, &uc.ContextID, &uc.Role,
			&uc.CreatedAt, &uc.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan user context: %w", err)
		}
		result = append(result, uc)
	}
	return result, nil
}

func (r *userPostgresRepo) CreateMembership(uc *domainctx.UserContext) error {
	if uc.ID == "" {
		uc.ID = uuid.New().String()
	}
	_, err := r.pool.Exec(context.Background(), `
		INSERT INTO user_contexts (id, user_id, context_id, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
	`, uc.ID, uc.UserID, uc.ContextID, uc.Role)
	if err != nil {
		return fmt.Errorf("create user context membership: %w", err)
	}
	return nil
}

func (r *userPostgresRepo) UpdateMembership(uc *domainctx.UserContext) error {
	result, err := r.pool.Exec(context.Background(), `
		UPDATE user_contexts SET role = $1, updated_at = NOW()
		WHERE user_id = $2 AND context_id = $3
	`, uc.Role, uc.UserID, uc.ContextID)
	if err != nil {
		return fmt.Errorf("update user context membership: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("user context membership not found")
	}
	return nil
}

func (r *userPostgresRepo) DeleteMembership(userID, contextID string) error {
	result, err := r.pool.Exec(context.Background(), `
		DELETE FROM user_contexts WHERE user_id = $1 AND context_id = $2
	`, userID, contextID)
	if err != nil {
		return fmt.Errorf("delete user context membership: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("user context membership not found")
	}
	return nil
}

func (r *userPostgresRepo) ListMembers(contextID string) ([]domainctx.UserContext, error) {
	rows, err := r.pool.Query(context.Background(), `
		SELECT id, user_id, context_id, role, created_at, updated_at
		FROM user_contexts WHERE context_id = $1
	`, contextID)
	if err != nil {
		return nil, fmt.Errorf("list context members: %w", err)
	}
	defer rows.Close()

	var result []domainctx.UserContext
	for rows.Next() {
		var uc domainctx.UserContext
		if err := rows.Scan(&uc.ID, &uc.UserID, &uc.ContextID, &uc.Role,
			&uc.CreatedAt, &uc.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan user context: %w", err)
		}
		result = append(result, uc)
	}
	return result, nil
}

// UpdateProfile updates user display name and/or avatar URL.
func (r *userPostgresRepo) UpdateProfile(userID string, displayName, avatarURL *string) error {
	ctx := context.Background()
	query := `
		UPDATE users SET display_name = COALESCE($2, display_name),
			avatar_url = COALESCE($3, avatar_url), updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, userID, displayName, avatarURL)
	if err != nil {
		return fmt.Errorf("update profile: %w", err)
	}
	return nil
}

// ChangePassword updates the user's password hash and sets password_changed_at.
func (r *userPostgresRepo) ChangePassword(userID string, passwordHash string) error {
	ctx := context.Background()
	query := `
		UPDATE users SET password_hash = $2, password_changed_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, userID, passwordHash)
	if err != nil {
		return fmt.Errorf("change password: %w", err)
	}
	return nil
}

// GetPasswordChangedAt returns the password_changed_at timestamp.
func (r *userPostgresRepo) GetPasswordChangedAt(userID string) (time.Time, error) {
	ctx := context.Background()
	var pwt time.Time
	query := "SELECT password_changed_at FROM users WHERE id = $1"
	err := r.pool.QueryRow(ctx, query, userID).Scan(&pwt)
	if err != nil {
		return time.Time{}, fmt.Errorf("get password changed at: %w", err)
	}
	return pwt, nil
}

// UpdatePasswordChangedAt sets password_changed_at to NOW().
func (r *userPostgresRepo) UpdatePasswordChangedAt(userID string) error {
	ctx := context.Background()
	query := `
		UPDATE users SET password_changed_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("update password changed at: %w", err)
	}
	return nil
}
