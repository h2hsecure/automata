package webui

import "github.com/hekemen/automata/internal/domain/context"

// UserRepository is an alias to the existing context.UserRepository
// interface so the webui package doesn't need its own dependency on
// context package internals. It's used by the auth handler to look up
// user details (is_admin flag) during login.
type UserRepository = context.UserRepository
