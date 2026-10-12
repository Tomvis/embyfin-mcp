package tools

import (
	"strings"

	"github.com/katbyte/go-kt/outside"
)

// Most of what a tool answers was written by someone else: a title and an overview from a metadata provider, a tag, a person's name, a file's name
// in a report, the words of a server's error. A model reads it as it finds it, so every piece of text in an answer, and in a refusal, has what does
// not show taken out of it and is cut when very long, once, around every tool, by go-kt's registry (queued, in all_tools.go). What is here is what
// the registry is told: how long, what to leave as it is, and where an address is quoted.

// answerLength is the most of one piece of text an answer carries. It is what the log tools keep of one line (lib/serverlog), the longest text a tool
// answers on purpose, so only text longer than anything a tool means to send is cut.
const answerLength = 16 << 10

// handles are the values in an answer that a later call hands back, or that name something on disk, by the names they are answered under: a path, a
// folder and a file; an id; a link to fetch; a setting's name; and a value an audit says to pass on exactly as held, spaces and all, with the lists of
// genres, tags and studios such a value is picked from. Each has to find the same thing again, so it is answered as the server holds it. A title or a
// name is not among them: it is shown cleaned, and the id beside it is what a later call takes.
//
// The last four are text of many lines, a log's and a failure's with its stack, which would lose its line breaks cleaned whole: it is cleaned a line
// at a time where it is read (lib/serverlog, and severalLines here).
var handles = []string{
	"path", "paths", "folder", "folders", "locations", "file", "files", "series_path", "removed_folders", "on_server", "examples", "written", "unnumbered_files",
	"id", "ids", "item_id", "series_id", "entry_id", "now_playing_id", "was_id", "was_all_ids", "metadata_provider_ids", "key",
	"image", "url", "editable", "setting",
	"value", "keep", "genres", "tags", "studios",
	"lines", "tail", "log_line", "error_detail",
}

// serverSaid are the values in an answer that carry a server's own words for what went wrong, or this program's with those words inside them, by the
// names they are answered under. Such words can quote the address that was asked with a key on the end of it, which was never the caller's to be
// shown, so an address under one of these names loses who it signs in as, the value of every parameter and what follows a #. An address anywhere
// else in an answer is a link to follow and is left; one in a refusal is blanked by the registry unasked.
var serverSaid = []string{"error", "errors", "batch_errors", "summary", "note", "warning"}

// severalLines is a server's text of many lines cleaned a line at a time, so that it keeps its line breaks: what does not show is taken out of each
// line, and an address in it loses what may carry a key.
func severalLines(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = outside.BlankAddresses(outside.Text(line, 0))
	}

	return strings.Join(lines, "\n")
}
