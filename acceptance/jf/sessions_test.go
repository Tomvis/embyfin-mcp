//go:build acceptance

package jf_test

import (
	"slices"
	"testing"

	"github.com/katbyte/embyfin-mcp/lib/sdkacc"

	"github.com/katbyte/embyfin-mcp/sdk/jf/12.1.0/jf"
	"github.com/katbyte/go-kt/pointer"
)

// TestJFSessions makes the suite's own session controllable and then sends
// it the three kinds of command. Nothing is listening on the session's end,
// so what is asserted is that the server accepts each command for a session
// that advertises support, and reports the capabilities back.
//

func TestJFSessions(t *testing.T) {
	ctx := skipUnlessJellyfin(t)
	movie := jfMovie(t, sdkacc.Alien)

	if _, err := jfc.Session.PostCapabilities(ctx, jf.PostCapabilitiesOptions{
		PlayableMediaTypes:   []jf.MediaType{jf.MediaTypeVideo},
		SupportedCommands:    []jf.GeneralCommandType{jf.GeneralCommandTypeDisplayMessage, jf.GeneralCommandTypePlay},
		SupportsMediaControl: new(true),
	}); err != nil {
		t.Fatal(err)
	}

	sessions := sdkacc.Must(jfc.Session.GetSessions(ctx, jf.GetSessionsOptions{DeviceId: sdkacc.DeviceID})).Answer
	i := slices.IndexFunc(sessions, func(s jf.SessionInfoDto) bool {
		return s.DeviceId == sdkacc.DeviceID && s.Capabilities != nil && pointer.From(s.Capabilities.SupportsMediaControl)
	})
	if i < 0 {
		t.Fatalf("no controllable session for device %q after PostCapabilities: %+v", sdkacc.DeviceID, sessions)
	}
	own := sessions[i]
	// the session's own SupportsMediaControl flag stays false until a
	// websocket controller attaches; the capabilities are what round-trip
	if own.SupportsMediaControl {
		t.Errorf("SupportsMediaControl is true for a session with no websocket: %+v", own)
	}
	if own.Id == "" || own.Client == "" || own.DeviceName == "" || own.LastActivityDate.IsZero() || own.ServerId == "" {
		t.Errorf("session = %+v", own)
	}
	if !slices.Contains(own.PlayableMediaTypes, jf.MediaTypeVideo) || !slices.Contains(own.SupportedCommands, jf.GeneralCommandTypeDisplayMessage) {
		t.Errorf("capabilities did not round-trip: %+v", own)
	}

	// the full list has every session, ours included
	all := sdkacc.Must(jfc.Session.GetSessions(ctx, jf.GetSessionsOptions{})).Answer
	if !slices.ContainsFunc(all, func(s jf.SessionInfoDto) bool { return s.Id == own.Id }) {
		t.Errorf("GetSessions() lacks the session %s that GetSessions(DeviceId) found", own.Id)
	}

	if _, err := jfc.Session.Play(ctx, own.Id, jf.PlayOptions{PlayCommand: jf.PlayCommandPlayNow, ItemIds: []jf.ItemID{movie.Id}}); err != nil {
		t.Errorf("Play: %v", err)
	}
	if _, err := jfc.Session.SendPlaystateCommand(ctx, own.Id, jf.PlaystateCommandPause, jf.SendPlaystateCommandOptions{}); err != nil {
		t.Errorf("SendPlaystateCommand(Pause): %v", err)
	}
	if _, err := jfc.Session.SendPlaystateCommand(ctx, own.Id, jf.PlaystateCommandSeek, jf.SendPlaystateCommandOptions{SeekPositionTicks: new(int64(10_000_000))}); err != nil {
		t.Errorf("SendPlaystateCommand(Seek): %v", err)
	}
	if _, err := jfc.Session.SendMessageCommand(ctx, own.Id, jf.MessageCommand{Header: "SDK", Text: "hello from the acceptance suite", TimeoutMs: 1000}); err != nil {
		t.Errorf("SendMessageCommand: %v", err)
	}

	// the full-capabilities body form
	if _, err := jfc.Session.PostFullCapabilities(ctx, jf.ClientCapabilitiesDto{
		PlayableMediaTypes: []jf.MediaType{jf.MediaTypeVideo}, SupportedCommands: []jf.GeneralCommandType{jf.GeneralCommandTypeDisplayMessage}, SupportsMediaControl: new(true),
	}); err != nil {
		t.Errorf("PostFullCapabilities: %v", err)
	}
}
