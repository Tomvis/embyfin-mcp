package embyfin

import (
	"testing"

	"github.com/katbyte/embyfin-mcp/sdk/emby/4.10.0.40/emby"
	"github.com/katbyte/embyfin-mcp/sdk/jf/12.1.0/jf"
)

// A date read from a server is carried as the text that server wrote, so that what a tool shows, compares and sorts is what it was before the SDKs
// held dates as times. Each form here is one a server was seen to write.
func TestADateIsCarriedAsTheServerWroteIt(t *testing.T) {
	t.Parallel()

	// Emby always writes seven digits, a trailing zero included
	for _, text := range []string{"2026-09-14T11:00:00.0000000Z", "2026-09-14T11:00:00.1234567Z", "2026-09-14T11:00:00.1234560Z", "2026-09-14T11:00:00.0001234Z"} {
		if got := embyDate(dateOf[emby.Time](text)); got != text {
			t.Errorf("Emby's %s is carried as %s", text, got)
		}
	}

	// Jellyfin writes seven digits when the first three are zero, and otherwise as many as the time has
	for _, text := range []string{"2026-09-14T11:00:00.0000000Z", "2026-09-14T11:00:00.0001234Z", "2026-09-14T11:00:00.1234567Z", "2026-09-14T11:00:00.123456Z", "2026-09-14T11:00:00.12345Z", "2026-09-14T11:00:00.123Z", "2026-09-14T11:00:00.05Z"} {
		if got := jfDate(dateOf[jf.Time](text)); got != text {
			t.Errorf("Jellyfin's %s is carried as %s", text, got)
		}
	}
}

// Year 1 is how both servers write a date they do not have, and it is carried as no date, the same as one left out. A date in no form the SDK
// reads is carried as the server sent it, so that a tool can still say what it was.
func TestADateTheServerDoesNotHaveIsNoDate(t *testing.T) {
	t.Parallel()

	for _, text := range []string{"", "0001-01-01T00:00:00.0000000Z", "0001-01-01T00:00:00Z"} {
		if got := embyDate(dateOf[emby.Time](text)); got != "" {
			t.Errorf("Emby's %q is carried as %q, want no date", text, got)
		}
		if got := jfDate(dateOf[jf.Time](text)); got != "" {
			t.Errorf("Jellyfin's %q is carried as %q, want no date", text, got)
		}
	}

	if got := embyDate(dateOf[emby.Time]("soon")); got != "soon" {
		t.Errorf("a date Emby wrote that cannot be read is carried as %q, want it as sent", got)
	}
	if got := jfDate(dateOf[jf.Time]("soon")); got != "soon" {
		t.Errorf("a date Jellyfin wrote that cannot be read is carried as %q, want it as sent", got)
	}
}
