//go:build acceptance

package emby_test

import (
	"context"
	"encoding/json"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/katbyte/embyfin-mcp/lib/sdkacc"

	"github.com/katbyte/embyfin-mcp/sdk/emby/4.10.0.40/emby"
	"github.com/katbyte/go-kt/pointer"
)

// embyPackages reads the package catalogue, which Emby fetches from its
// repository. The container asks for it as it starts, before the suite's
// provider proxy is listening, and a fetch that failed makes Emby refuse that
// host for half a minute or so after ("Cancelling connection ... due to a
// previous timeout", a 500), so the read is asked again until the refusal
// has passed. Any other failure is one.
func embyPackages(ctx context.Context, t *testing.T) []emby.PackageInfo {
	t.Helper()

	var (
		packages []emby.PackageInfo
		last     error
	)
	sdkacc.Poll(2*time.Minute, func() bool {
		res, err := embyc.Package.GetPackages(ctx, emby.GetPackagesOptions{})
		packages, last = res.Answer, err

		return err == nil || !strings.Contains(err.Error(), "due to a previous timeout")
	})
	if last != nil {
		t.Fatalf("GetPackages = %v", last)
	}
	if len(packages) == 0 {
		t.Fatal("GetPackages listed nothing")
	}

	return packages
}

func TestEmbySystem(t *testing.T) {
	ctx := skipUnlessEmby(t)

	// (LogPath and ProgramDataPath are null on this server)
	info := sdkacc.Must(embyc.System.GetSystemInfo(ctx)).Answer
	if info.Id == "" || info.Version == "" || info.ServerName == "" || info.OperatingSystem == "" || info.HttpServerPortNumber == 0 {
		t.Errorf("GetSystemInfo = %+v", info)
	}
	pub := sdkacc.Must(embyc.System.GetSystemInfoPublic(ctx)).Answer
	if pub.Id != info.Id || pub.Version != info.Version || pub.ServerName == "" {
		t.Errorf("GetSystemInfoPublic = %+v, want to match %+v", pub, info)
	}
	if ping := pointer.From(sdkacc.Must(embyc.System.GetSystemPing(ctx)).Answer); ping != "Emby Server" {
		t.Errorf("GetSystemPing = %q", ping)
	}
	if ep := sdkacc.Must(embyc.System.GetSystemEndpoint(ctx)).Answer; !ep.IsInNetwork {
		t.Errorf("GetSystemEndpoint = %+v, want IsInNetwork from the docker bridge", ep)
	}
	if _, err := embyc.System.GetSystemWakeOnLanInfo(ctx); err != nil {
		t.Errorf("GetSystemWakeOnLanInfo: %v", err)
	}
}

func TestEmbyLogsAndActivity(t *testing.T) {
	ctx := skipUnlessEmby(t)
	embyLibrary(t, sdkacc.SDKMovies) // so there is activity to list

	logs := sdkacc.Must(embyc.System.GetSystemLogsQuery(ctx, emby.GetSystemLogsQueryOptions{})).Answer.Items
	if len(logs) == 0 {
		t.Fatal("GetSystemLogsQuery listed nothing")
	}
	if logs[0].Name == "" || logs[0].Size == 0 || logs[0].DateModified.IsZero() {
		t.Errorf("log file = %+v", logs[0])
	}
	body := sdkacc.Must(embyc.System.GetSystemLogsByName(ctx, logs[0].Name)).HttpResponse.Body
	defer func() { _ = body.Close() }()
	if n, err := io.Copy(io.Discard, body); err != nil || n == 0 {
		t.Errorf("GetSystemLogsByName(%s) read %d bytes, err %v", logs[0].Name, n, err)
	}

	entries := sdkacc.Must(embyc.ActivityLog.GetSystemActivityLogEntries(ctx, emby.GetSystemActivityLogEntriesOptions{Limit: new(5)})).Answer
	if entries.TotalRecordCount == 0 || len(entries.Items) == 0 {
		t.Fatalf("GetSystemActivityLogEntries = %+v", entries)
	}
	for _, e := range entries.Items {
		if e.Id == 0 || e.Name == "" || e.Type == "" || e.Date.IsZero() || e.Severity == "" {
			t.Errorf("activity entry did not decode: %+v", e)
		}
	}

	// (TotalRecordCount is 0 whatever the list holds; a device is listed
	// once a user has authenticated from it, which an API key never does,
	// and the testenv script's login is always there)
	devices := sdkacc.Must(embyc.Device.GetDevices(ctx)).Answer
	if !slices.ContainsFunc(devices.Items, func(d emby.DevicesDeviceInfo) bool {
		return d.Name == "testenv" && d.Id != "" && d.AppName != "" && d.LastUserId == adminID && !d.DateLastActivity.IsZero()
	}) {
		t.Errorf("GetDevices does not list the testenv device: %+v", devices.Items)
	}
}

// The configuration, plugin and localisation reads a bare server answers.
//

func TestEmbyConfiguration(t *testing.T) {
	ctx := skipUnlessEmby(t)

	// (ServerName and MetadataPath are null while the defaults are in force)
	cfg := sdkacc.Must(embyc.Configuration.GetSystemConfiguration(ctx)).Answer
	if !pointer.From(cfg.IsStartupWizardCompleted) || cfg.UICulture == "" || cfg.HttpServerPortNumber == 0 || cfg.LogFileRetentionDays == 0 {
		t.Errorf("GetSystemConfiguration = %+v", cfg)
	}
	raw := sdkacc.Must(embyc.Configuration.GetSystemConfigurationByKey(ctx, "encoding")).Answer
	var encoding map[string]any
	if err := json.Unmarshal(raw, &encoding); err != nil || len(encoding) == 0 {
		t.Errorf("GetSystemConfigurationByKey(encoding) = %s, %v", raw, err)
	}
	if _, err := embyc.Branding.GetBrandingConfiguration(ctx); err != nil {
		t.Errorf("GetBrandingConfiguration: %v", err)
	}
	css := sdkacc.Must(embyc.Branding.GetBrandingCss(ctx))
	_ = css.HttpResponse.Body.Close()

	plugins := sdkacc.Must(embyc.Plugin.GetPlugins(ctx)).Answer
	if len(plugins) == 0 || plugins[0].Id == "" || plugins[0].Name == "" || plugins[0].Version == "" {
		t.Errorf("GetPlugins = %+v", plugins)
	}
	// the package catalogue is fetched from Emby's repository, through the
	// proxy
	if packages := embyPackages(ctx, t); len(packages) == 0 || packages[0].Name == "" || packages[0].Guid == "" {
		t.Errorf("GetPackages = %+v", packages)
	}

	cultures := sdkacc.Must(embyc.Localization.GetLocalizationCultures(ctx)).Answer
	if !slices.ContainsFunc(cultures, func(c emby.GlobalizationCultureDto) bool {
		return c.TwoLetterISOLanguageName == "en" && c.ThreeLetterISOLanguageName == "eng" && c.DisplayName != ""
	}) {
		t.Errorf("GetLocalizationCultures has no English among %d", len(cultures))
	}
	countries := sdkacc.Must(embyc.Localization.GetLocalizationCountries(ctx)).Answer
	if !slices.ContainsFunc(countries, func(c emby.GlobalizationCountryInfo) bool {
		return c.TwoLetterISORegionName == "US" && c.ThreeLetterISORegionName == "USA"
	}) {
		t.Errorf("GetLocalizationCountries has no US among %d", len(countries))
	}
	if ratings := sdkacc.Must(embyc.Localization.GetLocalizationParentalRatings(ctx)).Answer; len(ratings) == 0 || ratings[0].Name == "" {
		t.Errorf("GetLocalizationParentalRatings = %+v", ratings)
	}
	if opts := sdkacc.Must(embyc.Localization.GetLocalizationOptions(ctx)).Answer; !slices.ContainsFunc(opts, func(o emby.GlobalizationLocalizatonOption) bool { return o.Value == "en-US" }) {
		t.Errorf("GetLocalizationOptions has no en-US among %d", len(opts))
	}

	// the default browser path is empty on Linux; the call has to answer
	if _, err := embyc.Environment.GetEnvironmentDefaultDirectoryBrowser(ctx); err != nil {
		t.Errorf("GetEnvironmentDefaultDirectoryBrowser: %v", err)
	}
	contents := sdkacc.Must(embyc.Environment.GetEnvironmentDirectoryContents(ctx, emby.GetEnvironmentDirectoryContentsOptions{Path: "/media", IncludeDirectories: new(true), IncludeFiles: new(false)})).Answer
	if !slices.ContainsFunc(contents, func(e emby.IOFileSystemEntryInfo) bool { return e.Path == "/media/movies" && e.Type == "Directory" }) {
		t.Errorf("GetEnvironmentDirectoryContents(/media) = %+v", contents)
	}
	if drives := sdkacc.Must(embyc.Environment.GetEnvironmentDrives(ctx)).Answer; len(drives) == 0 {
		t.Error("GetEnvironmentDrives listed nothing")
	}
	// /Environment/ParentPath answers the bare path, not the JSON string the
	// spec promises, and labels it JSON all the same: the model is the text
	// as it was sent (the emby-parent-path-text workaround)
	if parent := pointer.From(sdkacc.Must(embyc.Environment.GetEnvironmentParentPath(ctx, "/media/movies")).Answer); parent != "/media" {
		t.Errorf("GetEnvironmentParentPath(/media/movies) = %q", parent)
	}

	prefs := sdkacc.Must(embyc.DisplayPreferences.GetDisplayPreferencesById(ctx, "usersettings", emby.GetDisplayPreferencesByIdOptions{UserId: adminID, Client: "emby"})).Answer
	if prefs.Id == "" {
		t.Errorf("GetDisplayPreferencesById = %+v", prefs)
	}

	// the Emby-only families a bare server can answer
	// (the server sends Name, Id and Events per category; the spec's Type
	// and Category are not among them)
	if types := sdkacc.Must(embyc.Notifications.GetNotificationsTypes(ctx)).Answer; len(types) == 0 || types[0].Name == "" {
		t.Errorf("GetNotificationsTypes = %+v", types)
	}
	if providers := sdkacc.Must(embyc.Sessions.GetAuthProviders(ctx)).Answer; len(providers) == 0 || providers[0].Id == "" {
		t.Errorf("GetAuthProviders = %+v", providers)
	}
	// the built-in folder targets are always offered
	if targets := sdkacc.Must(embyc.Sync.GetSyncTargets(ctx, adminID)).Answer; len(targets) == 0 || targets[0].Id == "" || targets[0].Name == "" {
		t.Errorf("GetSyncTargets = %+v", targets)
	}
	if jobs := sdkacc.Must(embyc.Sync.GetSyncJobs(ctx)).Answer; jobs.TotalRecordCount != 0 {
		t.Errorf("GetSyncJobs = %+v", jobs)
	}
	// Emby Connect is not linked: pending invitations are none
	if _, err := embyc.Connect.GetConnectPending(ctx); err != nil {
		t.Errorf("GetConnectPending: %v", err)
	}
	tv := sdkacc.Must(embyc.LiveTv.GetLiveTvInfo(ctx)).Answer
	if tv.IsEnabled {
		t.Errorf("GetLiveTvInfo = %+v on a server with no tuner", tv)
	}
	if types := sdkacc.Must(embyc.LiveTv.GetLiveTvTunerHostsTypes(ctx)).Answer; len(types) == 0 || types[0].Id == "" {
		t.Errorf("GetLiveTvTunerHostsTypes = %+v", types)
	}
	if _, err := embyc.LiveTv.GetLiveTvGuideInfo(ctx); err != nil {
		t.Errorf("GetLiveTvGuideInfo: %v", err)
	}
}
