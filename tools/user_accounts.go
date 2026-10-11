package tools

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/katbyte/embyfin-mcp/sdk/embyfin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// playlistsStay is what the account tools say of a deleted account's playlists.
const playlistsStay = "its playlists are not removed with it: they stay on the server, belonging to nobody, for the playlist tools to list and remove (seen on Emby 4.10 and Jellyfin 12.2)"

// leftoverNote is what user_create says of the watch state Jellyfin listed a new account with (see the client's CreateUser), "" when there was none.
func leftoverNote(left embyfin.Leftover) string {
	const seen = "Jellyfin lists a new account with the watched marks, favourites, ratings and resume points of an account deleted before it, until the server is restarted (seen on 12.2)"
	switch {
	case left.Unchecked != "":
		return fmt.Sprintf("%s, and this account could not be cleared of them: %s. Until Jellyfin is restarted, what a tool reads of its watch state from a list may be another account's", seen, left.Unchecked)
	case left.Cleared > 0:
		return fmt.Sprintf("%s: this one was listed with such state on %d of the items it sees, each cleared again and read again to see it gone", seen, left.Cleared)
	}

	return ""
}

// accountRow is an account as the tools that make and change one answer with it: who it is and what it may see.
type accountRow struct {
	Name         string   `json:"name"`
	ID           string   `json:"id"`
	Admin        bool     `json:"admin"`
	Hidden       bool     `json:"hidden"        jsonschema:"left off the login screen, where the name has to be typed; Jellyfin makes every new account so"`
	RemoteAccess bool     `json:"remote_access" jsonschema:"may connect from outside the local network; both servers make a new account so"`
	Libraries    []string `json:"libraries"     jsonschema:"the libraries the account can see; every library when all_libraries"`
	AllLibraries bool     `json:"all_libraries"`
}

func accountOf(u *embyfin.User, folders []embyfin.VirtualFolder) accountRow {
	row := accountRow{Name: u.Name, ID: u.ID, Admin: u.Policy.IsAdministrator, Hidden: u.Policy.IsHidden, RemoteAccess: u.Policy.EnableRemoteAccess, AllLibraries: u.Policy.EnableAllFolders, Libraries: []string{}}
	for i := range folders {
		if u.CanSee(&folders[i]) {
			row.Libraries = append(row.Libraries, folders[i].Name)
		}
	}

	return row
}

// librariesNamed finds the libraries asked for, by name or id, each once and in the order asked: one that is not there is an error naming the ones
// that are, since neither server checks what an account is given.
func librariesNamed(folders []embyfin.VirtualFolder, asked []string) ([]embyfin.VirtualFolder, error) {
	var picked []embyfin.VirtualFolder
	for _, want := range asked {
		want = strings.TrimSpace(want)
		at := slices.IndexFunc(folders, func(f embyfin.VirtualFolder) bool {
			return strings.EqualFold(f.Name, want) || (want != "" && (strings.EqualFold(f.ItemID, want) || strings.EqualFold(f.GUID, want)))
		})
		if at < 0 {
			names := make([]string, 0, len(folders))
			for i := range folders {
				names = append(names, folders[i].Name)
			}

			return nil, fmt.Errorf("no library named %q or with that id: the server has %s", want, strings.Join(names, ", "))
		}
		if !slices.ContainsFunc(picked, func(f embyfin.VirtualFolder) bool { return f.Name == folders[at].Name }) {
			picked = append(picked, folders[at])
		}
	}

	return picked, nil
}

// accountNamed finds the account a write names. A write never falls back on the first administrator, as the lookups do.
func accountNamed(ctx context.Context, client *embyfin.Client, user string) (*embyfin.User, error) {
	if strings.TrimSpace(user) == "" {
		return nil, errors.New("user is required: the account's name or id")
	}

	return client.ResolveUser(ctx, strings.TrimSpace(user))
}

func registerAccountTools(r *registry) {
	client := r.client

	type createIn struct {
		Name      string   `json:"name"                jsonschema:"the new account's name, which it logs in with"`
		Password  string   `json:"password"            jsonschema:"its password. Required: an account made with none can be logged into by anyone who knows its name"`
		Libraries []string `json:"libraries,omitempty" jsonschema:"the libraries it may see, by name or id, and no others. Empty leaves it every library, which is what both servers give a new account"`
	}
	type createOut struct {
		accountRow
		Note string `json:"note,omitempty"`
	}
	add(r, writeTool, &mcp.Tool{
		Name: "user_create",
		Description: "Make a new account with a password: an ordinary one, no administrator, which both servers let see every library and connect from outside the network unless libraries narrows what it sees. " +
			"A password is required, because Emby makes an account with none and both servers let one with none be logged into by its name alone. On Emby the password is set in a second step: if that fails, the account just made is removed again and the call is an error. " +
			"A name another account has, in any case, is refused before anything is sent; Jellyfin also refuses some characters in a name (a slash, for one) without saying which. Jellyfin leaves a new account off the login screen (hidden), where its name is typed. " +
			"Jellyfin 12.2 also lists a new account with the watch state of an account deleted before it, so on Jellyfin everything the new account sees is read once and anything it is listed as having watched, favourited or rated is cleared; note says when there was any, or when that could not be done. " +
			"The answer never holds the password. Change the account afterwards with user_edit.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createIn) (*mcp.CallToolResult, createOut, error) {
		name := strings.TrimSpace(in.Name)
		if name == "" {
			return nil, createOut{}, errors.New("name is required")
		}
		if in.Password == "" {
			return nil, createOut{}, embyfin.ErrNoPassword
		}

		users, err := client.Users(ctx)
		if err != nil {
			return nil, createOut{}, fmt.Errorf("could not read the accounts there are, so none was made: %w", err)
		}
		if at := slices.IndexFunc(users, func(u embyfin.User) bool { return strings.EqualFold(u.Name, name) }); at >= 0 {
			return nil, createOut{}, fmt.Errorf("an account named %q exists (id %s), and both servers refuse a name that differs from one in use only by case. Nothing was made: change that account with user_edit", users[at].Name, users[at].ID)
		}
		folders, err := client.VirtualFolders(ctx)
		if err != nil {
			return nil, createOut{}, fmt.Errorf("could not read the libraries, so no account was made: %w", err)
		}
		libraries, err := librariesNamed(folders, in.Libraries)
		if err != nil {
			return nil, createOut{}, fmt.Errorf("%w. No account was made", err)
		}

		user, left, err := client.CreateUser(ctx, name, in.Password)
		if err != nil {
			if client.Backend() == embyfin.Jellyfin && strings.Contains(err.Error(), "Error processing request") {
				return nil, createOut{}, fmt.Errorf("the name %q was refused by Jellyfin, which does not say why: it refuses some characters in a name (a slash, for one). Nothing was made: %w", name, err)
			}

			return nil, createOut{}, err
		}
		if len(libraries) > 0 {
			_, narrowed, err := client.SetUserLibraries(ctx, user.ID, false, libraries)
			if err != nil {
				return nil, createOut{}, fmt.Errorf("the account %s was made (id %s) and sees EVERY library: giving it only the ones asked for failed. Set them with user_edit, or remove it with user_delete: %w", user.Name, user.ID, err)
			}
			user = narrowed
		}

		out := createOut{accountRow: accountOf(user, folders), Note: leftoverNote(left)}
		if user.Policy.EnableRemoteAccess {
			out.Note = joinNotes("it may connect from outside the local network, as both servers let a new account", out.Note)
		}

		return nil, out, nil
	})

	type editIn struct {
		User         string   `json:"user"                    jsonschema:"the account, by name or id"`
		Password     string   `json:"password,omitempty"      jsonschema:"a new password, set with this server key's right to and without the old one"`
		Libraries    []string `json:"libraries,omitempty"     jsonschema:"the libraries it may see, by name or id: exactly these, in place of what it saw"`
		AllLibraries bool     `json:"all_libraries,omitempty" jsonschema:"true lets it see every library, those added later included"`
	}
	type accountWas struct {
		Libraries    []string `json:"libraries"`
		AllLibraries bool     `json:"all_libraries"`
	}
	type editOut struct {
		accountRow
		Changed []string    `json:"changed"        jsonschema:"what was changed: password, libraries"`
		Was     *accountWas `json:"was,omitempty"  jsonschema:"the libraries it saw before, when they were changed, to set them back by"`
		Note    string      `json:"note,omitempty"`
	}
	add(r, writeTool, &mcp.Tool{
		Name: "user_edit",
		Description: "Change an account: give it a new password, or say which libraries it sees - exactly the ones named, or every one with all_libraries. " +
			"The libraries are checked against the ones the server has before anything is sent, since neither server does: an id that names no library is kept as given, and the account then sees nothing by it. " +
			"An account's password cannot be taken away here: an empty one does that on both servers, and is never sent. was in the answer holds the libraries it saw before; the password is never in an answer. " +
			"An administrator narrowed to some libraries no longer reads every list whole, which the tools that need an account seeing everything say when it matters.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in editIn) (*mcp.CallToolResult, editOut, error) {
		user, err := accountNamed(ctx, client, in.User)
		if err != nil {
			return nil, editOut{}, err
		}
		switch {
		case in.Password == "" && len(in.Libraries) == 0 && !in.AllLibraries:
			return nil, editOut{}, errors.New("nothing to change: give password, libraries or all_libraries")
		case len(in.Libraries) > 0 && in.AllLibraries:
			return nil, editOut{}, errors.New("libraries and all_libraries are two answers to one question: give one")
		}
		folders, err := client.VirtualFolders(ctx)
		if err != nil {
			return nil, editOut{}, fmt.Errorf("could not read the libraries, so nothing was changed: %w", err)
		}
		libraries, err := librariesNamed(folders, in.Libraries)
		if err != nil {
			return nil, editOut{}, fmt.Errorf("%w. Nothing was changed", err)
		}

		out := editOut{Changed: []string{}}
		if len(libraries) > 0 || in.AllLibraries {
			before, after, err := client.SetUserLibraries(ctx, user.ID, in.AllLibraries, libraries)
			if err != nil {
				return nil, editOut{}, err
			}
			was := accountOf(before, folders)
			out.Was = &accountWas{Libraries: was.Libraries, AllLibraries: was.AllLibraries}
			out.Changed = append(out.Changed, "libraries")
			user = after
		}
		if in.Password != "" {
			if err := client.SetUserPassword(ctx, user.ID, in.Password); err != nil {
				if len(out.Changed) > 0 {
					return nil, editOut{}, fmt.Errorf("the libraries %s sees were changed, and its password was not: %w", user.Name, err)
				}

				return nil, editOut{}, err
			}
			out.Changed = append(out.Changed, "password")
		}
		out.accountRow = accountOf(user, folders)

		return nil, out, nil
	})

	type deleteIn struct {
		User    string `json:"user"              jsonschema:"the account, by name or id"`
		Confirm bool   `json:"confirm,omitempty" jsonschema:"must be true to delete; without it the call is refused with what the account is"`
	}
	type deleteOut struct {
		Deleted string `json:"deleted"`
		ID      string `json:"id"`
		Admin   bool   `json:"admin"`
		Note    string `json:"note"`
	}
	add(r, deleteTool, &mcp.Tool{
		Name: "user_delete",
		Description: "Delete an account, which cannot be undone: it can no longer log in, and an account made again by the same name is a new one, with a new id and none of the old one's watched marks, progress or favourites. " +
			"The server's last administrator is never deleted. An account's playlists are not removed with it: both servers keep them, belonging to nobody, for the playlist tools to list and remove. " +
			"On Jellyfin the delete waits three seconds first, and an account that is playing something is not deleted: Jellyfin 12.2 exits, the whole server, when an account is deleted within half a second of its watch state changing, as a watched mark does and playback does every few seconds. " +
			"Without confirm=true it refuses, saying what the account is and what it sees.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in deleteIn) (*mcp.CallToolResult, deleteOut, error) {
		user, err := accountNamed(ctx, client, in.User)
		if err != nil {
			return nil, deleteOut{}, err
		}
		users, err := client.Users(ctx)
		if err != nil {
			return nil, deleteOut{}, fmt.Errorf("could not read the accounts, so none was deleted: %w", err)
		}
		administrators := 0
		for i := range users {
			if users[i].Policy.IsAdministrator && !users[i].Policy.IsDisabled {
				administrators++
			}
		}
		if user.Policy.IsAdministrator && !user.Policy.IsDisabled && administrators <= 1 {
			return nil, deleteOut{}, fmt.Errorf("%s is the server's only administrator that is not disabled: deleting it would leave nobody able to run the server. Nothing was deleted", user.Name)
		}

		if !in.Confirm {
			folders, err := client.VirtualFolders(ctx)
			if err != nil {
				return nil, deleteOut{}, fmt.Errorf("could not read the libraries, and nothing was deleted: %w", err)
			}
			row, is, sees := accountOf(user, folders), "an ordinary account", "every library"
			if row.Admin {
				is = "an administrator"
			}
			if !row.AllLibraries {
				sees = "no library"
				if len(row.Libraries) > 0 {
					sees = strings.Join(row.Libraries, ", ")
				}
			}
			active := "it has never been active"
			if user.LastActivityDate != "" {
				active = "it was last active " + user.LastActivityDate
			}

			return nil, deleteOut{}, fmt.Errorf("refusing to delete the account %s (id %s) without confirm=true: nothing was changed. It is %s that sees %s; %s. Its watched marks, progress and favourites go with it; %s", user.Name, user.ID, is, sees, active, playlistsStay)
		}

		if err := client.DeleteUser(ctx, user.ID); err != nil {
			return nil, deleteOut{}, err
		}

		return nil, deleteOut{Deleted: user.Name, ID: user.ID, Admin: user.Policy.IsAdministrator, Note: playlistsStay}, nil
	})
}
