//go:build acceptance

package tmdb_test

// TMDB's read surface, swept the way the servers' are: every GET in its
// definitions called against the real API through the recording proxy, and
// its answer decoded into the generated model. That is what shows that the
// definitions, drawn from the examples in TMDB's own documentation, are right
// about what TMDB answers.
//
// TMDB is not a server the suite runs, so this needs no container and keeps
// its own cassettes: the answers are recorded once with a token
// (EMBYFIN_TEST_RECORD=1 and EMBYFIN_TMDB_TOKEN) into testdata/cassettes/tmdb,
// and replayed after with no token and no network.

import (
	"cmp"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/katbyte/embyfin-mcp/lib/sdkacc"

	"github.com/katbyte/embyfin-mcp/lib/testenv"
	tmdbclient "github.com/katbyte/embyfin-mcp/sdk/tmdb/2026.09.22/client"
	"github.com/katbyte/embyfin-mcp/sdk/tmdb/2026.09.22/tmdb"
	"github.com/katbyte/go-kt/test/replayproxy"
	"github.com/katbyte/pandorest/sweep"
)

// The ids the sweep starts from, which TMDB's own documentation uses: Fight
// Club, Game of Thrones, Brad Pitt and the Star Wars collection.
const (
	tmdbFilm       = 550
	tmdbSeries     = 1399
	tmdbPerson     = 287
	tmdbCollection = 10
)

// tmdbCases are the GETs the sweep does not call, and why.
var tmdbCases = func() map[string]sweep.Case {
	session := sweep.Case{Skip: "needs a signed-in user's session, and answers with that user's account: nothing the suite may hold or publish"}
	cases := map[string]sweep.Case{}
	for _, name := range []string{
		"AccountDetails", "AccountFavoriteTv", "AccountGetFavorites", "AccountLists", "AccountRatedMovies", "AccountRatedTv",
		"AccountRatedTvEpisodes", "AccountWatchlistMovies", "AccountWatchlistTv",
		"MovieAccountStates", "TvEpisodeAccountStates", "TvSeasonAccountStates", "TvSeriesAccountStates",
	} {
		cases[name] = session
	}
	// the suite rates nothing on TMDB, where a guest's rating counts toward
	// the film's, and a guest session that has rated nothing is not found
	for _, name := range []string{"GuestSessionRatedMovies", "GuestSessionRatedTv", "GuestSessionRatedTvEpisodes"} {
		cases[name] = sweep.Case{Status: http.StatusNotFound, Why: "a guest session that has rated nothing answers 404"}
	}
	// a changes route lists the last day's, and the day these were recorded
	// the person and the episode had none
	cases["PersonChanges"] = sweep.Case{Empty: "Brad Pitt had no change the day it was recorded"}
	cases["TvEpisodeChangesById"] = sweep.Case{Empty: "the episode had no change the day it was recorded"}

	return cases
}()

func TestTMDBSweep(t *testing.T) {
	mode, token := replayproxy.Replay, "replay" // the proxy leaves api_key out of a match
	switch {
	case testenv.Recording(), testenv.Verifying():
		token = cmp.Or(os.Getenv("EMBYFIN_TMDB_TOKEN"), os.Getenv("EMBYFIN_TMDB_KEY"))
		if token == "" {
			t.Skip("recording TMDB needs EMBYFIN_TMDB_TOKEN")
		}
		// what is missing, or everything afresh (EMBYFIN_TEST_RECORD=all)
		mode = testenv.Mode()
		if testenv.Verifying() {
			mode = replayproxy.Verify
		}
	}
	p, err := replayproxy.New(replayproxy.Options{
		Mode:        mode,
		CassetteDir: filepath.Join("testdata", "cassettes", "tmdb"),
		Addr:        "127.0.0.1:0",
		RedactQuery: []string{"api_key"},
		// a new request token is a credential for a moment, and the cassette
		// should not hold even that
		RedactBodyFields: []string{"request_token"},
		Logger:           log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		misses, drifts := p.Misses(), p.Drifts()
		_ = p.Close()
		if len(misses) > 0 {
			t.Errorf("%d request(s) had no recording; record them with EMBYFIN_TEST_RECORD=1 EMBYFIN_TMDB_TOKEN=...: %v", len(misses), misses)
		}
		for _, d := range drifts {
			t.Errorf("TMDB's answer changed shape since it was recorded: %s", d)
		}
	})

	c, err := tmdbclient.New(tmdb.DefaultBaseURL, token)
	if err != nil {
		t.Fatal(err)
	}
	c.Client.HTTPClient = &http.Client{Timeout: time.Minute, Transport: p.Transport()}

	sdkacc.RunSweep(t, "tmdb", c, tmdbFixtures(t, c), tmdbCases, nil)
}

// tmdbFixtures resolves the ids no example in the document gives: the fixed
// ones above, and the rest read off TMDB's own answers about them.
func tmdbFixtures(t *testing.T, c *tmdbclient.Client) sweep.Fixtures {
	t.Helper()

	ctx := t.Context()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	id := strconv.Itoa

	film, err := c.Movie.MovieDetails(ctx, tmdbFilm, tmdb.MovieDetailsOptions{})
	must(err)
	credits, err := c.Movie.MovieCredits(ctx, tmdbFilm)
	must(err)
	reviews, err := c.Movie.MovieReviews(ctx, tmdbFilm, tmdb.MovieReviewsOptions{})
	must(err)
	lists, err := c.Movie.MovieLists(ctx, tmdbFilm, tmdb.MovieListsOptions{})
	must(err)
	keywords, err := c.Movie.MovieKeywords(ctx, tmdbFilm)
	must(err)
	series, err := c.TV.TvSeriesDetails(ctx, tmdbSeries, tmdb.TvSeriesDetailsOptions{})
	must(err)
	season, err := c.TV.TvSeasonDetails(ctx, tmdbSeries, 1, tmdb.TvSeasonDetailsOptions{})
	must(err)
	episode, err := c.TV.TvEpisodeDetails(ctx, tmdbSeries, 1, 1, tmdb.TvEpisodeDetailsOptions{})
	must(err)
	groups, err := c.TV.TvSeriesEpisodeGroups(ctx, tmdbSeries)
	must(err)
	guest, err := c.Authentication.AuthenticationCreateGuestSession(ctx)
	must(err)

	if len(film.Answer.ProductionCompanies) == 0 || len(credits.Answer.Cast) == 0 || len(reviews.Answer.Results) == 0 || len(lists.Answer.Results) == 0 ||
		len(keywords.Answer.Keywords) == 0 || len(series.Answer.Networks) == 0 || len(groups.Answer.Results) == 0 {
		t.Fatal("TMDB's answers about the starting ids no longer hold the ids the sweep reads off them")
	}

	return sweep.Fixtures{
		Path: map[string]string{
			"movie_id":            id(tmdbFilm),
			"series_id":           id(tmdbSeries),
			"season_number":       "1",
			"episode_number":      "1",
			"person_id":           id(tmdbPerson),
			"collection_id":       id(tmdbCollection),
			"company_id":          id(film.Answer.ProductionCompanies[0].Id),
			"network_id":          id(series.Answer.Networks[0].Id),
			"keyword_id":          id(keywords.Answer.Keywords[0].Id),
			"credit_id":           credits.Answer.Cast[0].CreditId,
			"review_id":           reviews.Answer.Results[0].Id,
			"list_id":             id(lists.Answer.Results[0].Id),
			"season_id":           id(season.Answer.Id),
			"episode_id":          id(episode.Answer.Id),
			"tv_episode_group_id": groups.Answer.Results[0].Id,
			"guest_session_id":    guest.Answer.GuestSessionId,
			"external_id":         film.Answer.ImdbId,
			"time_window":         "day",
		},
		Options: map[string]string{
			"query":           "star wars",
			"external_source": "imdb_id",
		},
	}
}
