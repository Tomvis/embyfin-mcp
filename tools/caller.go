package tools

import (
	"context"
	"fmt"

	"github.com/katbyte/embyfin-mcp/sdk/embyfin"
)

// resolveUser is the account a tool acting for a user acts for. An empty
// name is Options.User when set, else the first administrator. With
// Options.UserOnly, naming any other account is refused: one instance per
// person, so one person's tools cannot read another's watching.
func (r *registry) resolveUser(ctx context.Context, nameOrID string) (*embyfin.User, error) {
	if r.opts.User == "" {
		return r.client.ResolveUser(ctx, nameOrID)
	}
	own, err := r.client.ResolveUser(ctx, r.opts.User)
	if err != nil {
		return nil, fmt.Errorf("the configured user: %w", err)
	}
	if nameOrID == "" {
		return own, nil
	}
	u, err := r.client.ResolveUser(ctx, nameOrID)
	if err != nil {
		return nil, err
	}
	if r.opts.UserOnly && u.ID != own.ID {
		return nil, fmt.Errorf("this server acts only for %s", own.Name)
	}

	return u, nil
}

// visibleUsers narrows a list of accounts to the configured user under
// Options.UserOnly, and leaves it whole otherwise.
func (r *registry) visibleUsers(ctx context.Context, users []embyfin.User) ([]embyfin.User, error) {
	if !r.opts.UserOnly || r.opts.User == "" {
		return users, nil
	}
	own, err := r.client.ResolveUser(ctx, r.opts.User)
	if err != nil {
		return nil, fmt.Errorf("the configured user: %w", err)
	}
	for i := range users {
		if users[i].ID == own.ID {
			return users[i : i+1], nil
		}
	}

	return []embyfin.User{}, nil
}
