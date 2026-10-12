//go:build acceptance

package emby_test

import (
	"slices"
	"strconv"
	"testing"

	"github.com/katbyte/embyfin-mcp/lib/sdkacc"

	"github.com/katbyte/embyfin-mcp/sdk/emby/4.10.0.40/emby"
)

// TestEmbySessions makes the suite's own session controllable and then sends
// it the three kinds of command. Nothing is listening on the session's end,
// so what is asserted is that the server accepts each command for a session
// that advertises support, and reports the capabilities back.
//

func TestEmbySessions(t *testing.T) {
	ctx := skipUnlessEmby(t)
	movie := embyMovie(t, sdkacc.Alien)
	itemID, err := strconv.Atoi(string(movie.Id))
	if err != nil {
		t.Fatalf("item id %q is not an integer: %v", movie.Id, err)
	}

	if _, err := embyc.Sessions.PostSessionsCapabilities(ctx, emby.PostSessionsCapabilitiesOptions{
		PlayableMediaTypes:   "Video",
		SupportedCommands:    "DisplayMessage,Play",
		SupportsMediaControl: new(true),
	}); err != nil {
		t.Fatal(err)
	}

	// the session's SupportsRemoteControl flag stays false until a websocket
	// controller attaches; the capabilities are what round-trip
	// the API key's session reports the key's device rather than the
	// DeviceId the client header names, so the suite's own session is the
	// one carrying the capabilities just posted
	sessions := sdkacc.Must(embyc.Sessions.GetSessions(ctx, emby.GetSessionsOptions{})).Answer
	i := slices.IndexFunc(sessions, func(s emby.SessionSessionInfo) bool {
		return slices.Contains(s.PlayableMediaTypes, "Video") && slices.Contains(s.SupportedCommands, "Play")
	})
	if i < 0 {
		t.Fatalf("no session carries the capabilities after PostSessionsCapabilities: %+v", sessions)
	}
	own := sessions[i]
	if own.Id == "" || own.Client == "" || own.DeviceName == "" || own.LastActivityDate.IsZero() || own.ServerId == "" {
		t.Errorf("session = %+v", own)
	}
	if !slices.Contains(own.PlayableMediaTypes, "Video") || !slices.Contains(own.SupportedCommands, "DisplayMessage") {
		t.Errorf("capabilities did not round-trip: %+v", own)
	}

	byDevice := sdkacc.Must(embyc.Sessions.GetSessions(ctx, emby.GetSessionsOptions{DeviceId: own.DeviceId})).Answer
	if !slices.ContainsFunc(byDevice, func(s emby.SessionSessionInfo) bool { return s.Id == own.Id }) {
		t.Errorf("GetSessions(DeviceId=%s) lacks the session %s", own.DeviceId, own.Id)
	}

	if _, err := embyc.Sessions.PostSessionsByIdPlaying(ctx, own.Id, emby.PlayRequest{}, emby.PostSessionsByIdPlayingOptions{PlayCommand: "PlayNow", ItemIds: []int64{int64(itemID)}}); err != nil {
		t.Errorf("play: %v", err)
	}
	if _, err := embyc.Sessions.PostSessionsByIdPlayingByCommand(ctx, own.Id, "Pause", emby.PlaystateRequest{}); err != nil {
		t.Errorf("playstate Pause: %v", err)
	}
	if _, err := embyc.Sessions.PostSessionsByIdPlayingByCommand(ctx, own.Id, "Seek", emby.PlaystateRequest{Command: "Seek", SeekPositionTicks: 10_000_000}); err != nil {
		t.Errorf("playstate Seek: %v", err)
	}
	// the start: the body leaves a position of 0 out, so the query carries it
	// (emby-playstate-seek-query), where Emby's own web client sends it
	if _, err := embyc.Sessions.PostSessionsByIdPlayingByCommand(ctx, own.Id, "Seek", emby.PlaystateRequest{Command: "Seek"}, 0); err != nil {
		t.Errorf("playstate Seek to the start: %v", err)
	}
	if _, err := embyc.Sessions.PostSessionsByIdMessage(ctx, own.Id, emby.PostSessionsByIdMessageOptions{Header: "SDK", Text: "hello from the acceptance suite", TimeoutMs: new(int64(1000))}); err != nil {
		t.Errorf("message: %v", err)
	}

	// the full-capabilities body form
	if _, err := embyc.Sessions.PostSessionsCapabilitiesFull(ctx, emby.ClientCapabilities{
		PlayableMediaTypes: []string{"Video"}, SupportedCommands: []string{"DisplayMessage"}, SupportsMediaControl: new(true),
	}, ""); err != nil {
		t.Errorf("full capabilities: %v", err)
	}
}
