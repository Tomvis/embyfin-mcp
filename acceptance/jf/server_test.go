//go:build acceptance

package jf_test

import (
	"io"
	"slices"
	"testing"

	"github.com/katbyte/embyfin-mcp/lib/sdkacc"

	"github.com/katbyte/embyfin-mcp/sdk/jf/12.1.0/jf"
	"github.com/katbyte/go-kt/pointer"
	"github.com/katbyte/pandorest/client"
)

func TestJFSystem(t *testing.T) {
	ctx := skipUnlessJellyfin(t)

	// (OperatingSystem is empty on this server; the paths and ids are not)
	info := sdkacc.Must(jfc.System.GetSystemInfo(ctx)).Answer
	if info.Id == "" || info.Version == "" || info.ServerName == "" || info.LogPath == "" || info.ProgramDataPath == "" || info.LocalAddress == "" {
		t.Errorf("GetSystemInfo = %+v", info)
	}
	if !pointer.From(info.StartupWizardCompleted) {
		t.Error("StartupWizardCompleted is false after testenv.sh completed the wizard")
	}
	pub := sdkacc.Must(jfc.System.GetPublicSystemInfo(ctx)).Answer
	if pub.Id != info.Id || pub.Version != info.Version || pub.ServerName == "" || pub.ProductName == "" {
		t.Errorf("GetPublicSystemInfo = %+v, want to match %+v", pub, info)
	}
	if ping := pointer.From(sdkacc.Must(jfc.System.GetPingSystem(ctx)).Answer); ping != "Jellyfin Server" {
		t.Errorf("GetPingSystem = %q", ping)
	}
	if ping := pointer.From(sdkacc.Must(jfc.System.PostPingSystem(ctx)).Answer); ping != "Jellyfin Server" {
		t.Errorf("PostPingSystem = %q", ping)
	}
	if ep := sdkacc.Must(jfc.System.GetEndpointInfo(ctx)).Answer; !ep.IsInNetwork {
		t.Errorf("GetEndpointInfo = %+v, want IsInNetwork from the docker bridge", ep)
	}
	// the storage report: the server's own folders with the space on their
	// drives, and each library's folders
	moviesID := jfLibrary(t, sdkacc.SDKMovies)
	storage := sdkacc.Must(jfc.System.GetSystemStorage(ctx)).Answer
	if storage.ProgramDataFolder == nil || storage.ProgramDataFolder.Path != "/config" || storage.ProgramDataFolder.FreeSpace == 0 || storage.ProgramDataFolder.UsedSpace == 0 ||
		storage.WebFolder == nil || storage.WebFolder.Path == "" || storage.LogFolder == nil || storage.LogFolder.Path == "" {
		t.Errorf("GetSystemStorage = %+v", storage)
	}
	if !slices.ContainsFunc(storage.Libraries, func(l jf.LibraryStorageDto) bool {
		return l.Id == string(moviesID) && l.Name == sdkacc.SDKMovies.Name && len(l.Folders) == 1 && l.Folders[0].Path == sdkacc.SDKMovies.Folder
	}) {
		t.Errorf("GetSystemStorage lists the libraries %+v, want %s over %s", storage.Libraries, sdkacc.SDKMovies.Name, sdkacc.SDKMovies.Folder)
	}
}

func TestJFLogsAndActivity(t *testing.T) {
	ctx := skipUnlessJellyfin(t)
	jfLibrary(t, sdkacc.SDKMovies) // so there is activity to list

	logs := sdkacc.Must(jfc.System.GetServerLogs(ctx)).Answer
	if len(logs) == 0 {
		t.Fatal("GetServerLogs listed nothing")
	}
	if logs[0].Name == "" || logs[0].Size == 0 || logs[0].DateModified.IsZero() {
		t.Errorf("log file = %+v", logs[0])
	}
	body := sdkacc.Must(jfc.System.GetLogFile(ctx, logs[0].Name)).HttpResponse.Body
	defer func() { _ = body.Close() }()
	if n, err := io.Copy(io.Discard, body); err != nil || n == 0 {
		t.Errorf("GetLogFile(%s) read %d bytes, err %v", logs[0].Name, n, err)
	}

	entries := sdkacc.Must(jfc.System.GetLogEntries(ctx, jf.GetLogEntriesOptions{Limit: new(5)})).Answer
	if entries.TotalRecordCount == 0 || len(entries.Items) == 0 {
		t.Fatalf("GetLogEntries = %+v", entries)
	}
	for _, e := range entries.Items {
		if e.Id == 0 || e.Name == "" || e.Type == "" || e.Date.IsZero() || e.Severity == "" {
			t.Errorf("activity entry did not decode: %+v", e)
		}
	}

	devices := sdkacc.Must(jfc.Device.GetDevices(ctx)).Answer
	if devices.TotalRecordCount == 0 || len(devices.Items) == 0 {
		t.Fatalf("GetDevices = %+v", devices)
	}
	// a device is listed once a user has authenticated from it, which an
	// API key never does; the testenv script's login is always there
	if !slices.ContainsFunc(devices.Items, func(d jf.DeviceInfoDto) bool {
		return d.Id == "testenv" && d.AppName != "" && d.LastUserId == adminID && !d.DateLastActivity.IsZero()
	}) {
		t.Errorf("GetDevices does not list the testenv device: %+v", devices.Items)
	}
}

// The configuration, plugin and localisation reads a bare server answers.
//

func TestJFConfiguration(t *testing.T) {
	ctx := skipUnlessJellyfin(t)

	// (ServerName and MetadataPath are empty in the configuration when the
	// defaults are in force; the wizard flag and the plugin repository list
	// are what a fresh server fills in)
	cfg := sdkacc.Must(jfc.System.GetConfiguration(ctx)).Answer
	if !pointer.From(cfg.IsStartupWizardCompleted) || len(cfg.PluginRepositories) == 0 || cfg.LogFileRetentionDays == 0 {
		t.Errorf("GetConfiguration = %+v", cfg)
	}
	if raw := sdkacc.Must(jfc.System.GetNamedConfiguration(ctx, "encoding")).Answer; len(raw) < 2 {
		t.Errorf("GetNamedConfiguration(encoding) = %s", raw)
	}
	if opts := sdkacc.Must(jfc.System.GetDefaultMetadataOptions(ctx)).Answer; opts.ItemType != "" {
		t.Errorf("GetDefaultMetadataOptions = %+v, want the blank defaults", opts)
	}
	if _, err := jfc.Branding.GetBrandingOptions(ctx); err != nil {
		t.Errorf("GetBrandingOptions: %v", err)
	}
	css := sdkacc.Must(jfc.Branding.GetBrandingCss(ctx))
	_ = css.HttpResponse.Body.Close()

	plugins := sdkacc.Must(jfc.Plugin.GetPlugins(ctx)).Answer
	if !slices.ContainsFunc(plugins, func(p jf.PluginInfo) bool {
		return p.Name == "TMDb" && p.Id != "" && p.Version != "" && p.Status == jf.PluginStatusActive
	}) {
		t.Errorf("GetPlugins = %+v, want the bundled TMDb plugin active", plugins)
	}
	if repos := sdkacc.Must(jfc.Plugin.GetRepositories(ctx)).Answer; len(repos) == 0 || repos[0].Url == "" {
		t.Errorf("GetRepositories = %+v", repos)
	}
	// the package catalogue is fetched from the repository, through the proxy
	packages := sdkacc.Must(jfc.Plugin.GetPackages(ctx)).Answer
	if len(packages) == 0 || packages[0].Name == "" || len(packages[0].Versions) == 0 {
		t.Errorf("GetPackages = %d packages", len(packages))
	}

	cultures := sdkacc.Must(jfc.Localization.GetCultures(ctx)).Answer
	if !slices.ContainsFunc(cultures, func(c jf.CultureDto) bool {
		return c.TwoLetterISOLanguageName == "en" && c.ThreeLetterISOLanguageName == "eng" && c.DisplayName != ""
	}) {
		t.Errorf("GetCultures has no English among %d", len(cultures))
	}
	countries := sdkacc.Must(jfc.Localization.GetCountries(ctx)).Answer
	if !slices.ContainsFunc(countries, func(c jf.CountryInfo) bool {
		return c.TwoLetterISORegionName == "US" && c.ThreeLetterISORegionName == "USA"
	}) {
		t.Errorf("GetCountries has no US among %d", len(countries))
	}
	if ratings := sdkacc.Must(jfc.Localization.GetParentalRatings(ctx)).Answer; len(ratings) == 0 || ratings[0].Name == "" {
		t.Errorf("GetParentalRatings = %+v", ratings)
	}
	if opts := sdkacc.Must(jfc.Localization.GetLocalizationOptions(ctx)).Answer; !slices.ContainsFunc(opts, func(o jf.LocalizationOption) bool { return o.Value == "en-US" }) {
		t.Errorf("GetLocalizationOptions has no en-US among %d", len(opts))
	}

	// the default browser path is empty on Linux; the call has to answer
	if _, err := jfc.Environment.GetDefaultDirectoryBrowser(ctx); err != nil {
		t.Errorf("GetDefaultDirectoryBrowser: %v", err)
	}
	contents := sdkacc.Must(jfc.Environment.GetDirectoryContents(ctx, jf.GetDirectoryContentsOptions{Path: "/media", IncludeDirectories: new(true), IncludeFiles: new(false)})).Answer
	if !slices.ContainsFunc(contents, func(e jf.FileSystemEntryInfo) bool {
		return e.Path == "/media/movies" && e.Type == jf.FileSystemEntryTypeDirectory
	}) {
		t.Errorf("GetDirectoryContents(/media) = %+v", contents)
	}
	if drives := sdkacc.Must(jfc.Environment.GetDrives(ctx)).Answer; len(drives) == 0 {
		t.Error("GetDrives listed nothing")
	}
	if parent := pointer.From(sdkacc.Must(jfc.Environment.GetParentPath(ctx, "/media/movies")).Answer); parent != "/media" {
		t.Errorf("GetParentPath(/media/movies) = %q", parent)
	}
	if _, err := jfc.Environment.ValidatePath(ctx, jf.ValidatePathDto{Path: "/media/movies"}); err != nil {
		t.Errorf("ValidatePath(/media/movies): %v", err)
	}
	if _, err := jfc.Environment.ValidatePath(ctx, jf.ValidatePathDto{Path: "/media/does-not-exist"}); client.StatusCode(err) != 404 {
		t.Errorf("ValidatePath on a missing folder = %v, want a 404", err)
	}

	prefs := sdkacc.Must(jfc.DisplayPreference.GetDisplayPreferences(ctx, "usersettings", jf.GetDisplayPreferencesOptions{UserId: adminID, Client: "emby"})).Answer
	if prefs.Id == "" || prefs.Client != "emby" {
		t.Errorf("GetDisplayPreferences = %+v", prefs)
	}

	// the wizard is done: its endpoints refuse an ordinary caller. The
	// admin key is elevated, so the read still answers; assert the shape.
	if cfg := sdkacc.Must(jfc.Startup.GetStartupConfiguration(ctx)).Answer; cfg.UICulture == "" || cfg.MetadataCountryCode == "" { //nolint:staticcheck // deprecated in the document and still an operation the SDK serves, which is what this holds it to
		t.Errorf("GetStartupConfiguration = %+v", cfg)
	}

	// the Jellyfin-only families a bare server can answer. Quick connect is
	// on by default now, so initiating hands back a code; off, it is a 401.
	if pointer.From(sdkacc.Must(jfc.Authentication.GetQuickConnectEnabled(ctx)).Answer) {
		if qc := sdkacc.Must(jfc.Authentication.InitiateQuickConnect(ctx)).Answer; qc.Secret == "" || qc.Code == "" || qc.Authenticated {
			t.Errorf("InitiateQuickConnect = %+v", qc)
		}
	} else if _, err := jfc.Authentication.InitiateQuickConnect(ctx); client.StatusCode(err) != 401 {
		t.Errorf("InitiateQuickConnect with quick connect off = %v, want a 401", err)
	}
	if backups, err := jfc.Backup.ListBackups(ctx); err != nil || len(backups.Answer) != 0 {
		t.Errorf("ListBackups = %+v, %v", backups.Answer, err)
	}
	// live tv is "enabled" with the built-in service and no tuner behind it
	tv := sdkacc.Must(jfc.LiveTv.GetLiveTvInfo(ctx)).Answer
	if len(tv.Services) == 0 || tv.Services[0].Name == "" || len(tv.Services[0].Tuners) != 0 {
		t.Errorf("GetLiveTvInfo = %+v on a server with no tuner", tv)
	}
	if types := sdkacc.Must(jfc.LiveTv.GetTunerHostTypes(ctx)).Answer; len(types) == 0 || types[0].Id == "" {
		t.Errorf("GetTunerHostTypes = %+v", types)
	}
	if _, err := jfc.LiveTv.GetGuideInfo(ctx); err != nil {
		t.Errorf("GetGuideInfo: %v", err)
	}
}
