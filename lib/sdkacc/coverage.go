package sdkacc

import (
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/katbyte/pandorest/client"
	"github.com/katbyte/pandorest/config"
	"github.com/katbyte/pandorest/definitions"
	"github.com/katbyte/pandorest/generator"

	"github.com/katbyte/embyfin-mcp/sdk/zservices"
)

// loadDefinitions reads a service's checked-in definitions, from a suite's folder.
func loadDefinitions(service string) (*definitions.Service, error) {
	cfg, ok := config.Find(zservices.All, service)
	if !ok {
		return nil, fmt.Errorf("no service %q", service)
	}
	cfg, err := cfg.In(repoRoot).Resolve()
	if err != nil {
		return nil, err
	}

	return definitions.Load(cfg.Path(cfg.Definitions))
}

// sdkFields are the field or variable each service's client sits on in embyfin's code.
var sdkFields = map[string]string{"emby": "emby", "jellyfin": "jf", "tmdb": "api"}

// LeftToNeutralSuite are, per service, the writes embyfin's code sends that the tool suite does not reach and the sdk/embyfin suite
// does, with why: the tool suite excuses them, and the sdk/embyfin suite fails when it does not reach every one.
var LeftToNeutralSuite = map[string]map[string]string{}

// UsedOperations is the operations of a service that embyfin's own code calls: each generated method a hand-written, non-test Go file
// names through the service's client (generator.UncalledOn), so c.emby.Items.GetItems is Emby's GetItems and api.Movie.MovieDetails
// TMDB's MovieDetails. A paged list's Complete and Each count for its operation.
func UsedOperations(service string) (map[string]*definitions.Operation, error) {
	field, ok := sdkFields[service]
	if !ok {
		return nil, fmt.Errorf("no field is known for %s's client", service)
	}
	svc, err := loadDefinitions(service)
	if err != nil {
		return nil, err
	}
	uncalled, _, err := generator.UncalledOn(svc, repoRoot, field)
	if err != nil {
		return nil, err
	}

	never := map[string]bool{}
	for _, name := range uncalled {
		never[name] = true
	}
	used := map[string]*definitions.Operation{}
	for _, op := range svc.Operations() {
		names := []string{op.Name}
		if op.Pageable != nil {
			names = append(names, op.Name+"Complete", op.Name+"Each")
		}
		if slices.ContainsFunc(names, func(n string) bool { return !never[n] }) {
			used[op.Name] = op
		}
	}
	if len(used) == 0 {
		return nil, fmt.Errorf("no file calls a %s method through %s, so the scan is looking in the wrong place", service, field)
	}

	return used, nil
}

// Writes is the operations of used that are not GETs: the ones a live suite has to reach itself, since the SDK suites' sweep calls every
// GET of each service.
func Writes(used map[string]*definitions.Operation) map[string]*definitions.Operation {
	writes := map[string]*definitions.Operation{}
	for name, op := range used {
		if !strings.EqualFold(op.Method, http.MethodGet) {
			writes[name] = op
		}
	}

	return writes
}

// Unreached says which of ops no recorded call reached, each by name, method and path: what embyfin's code sends that a live run never
// did. excused names what the run leaves to another suite, with why; an excuse for an operation the run reached, or that is not among
// ops, is a line too.
func Unreached(ops map[string]*definitions.Operation, calls []client.Call, excused map[string]string) []string {
	called := map[string]bool{}
	for _, c := range calls {
		called[c.Operation] = true
	}

	var lines []string
	for _, name := range slices.Sorted(maps.Keys(ops)) {
		op := ops[name]
		switch {
		case called[name] && excused[name] != "":
			lines = append(lines, fmt.Sprintf("%s (%s %s) was reached, so drop its excuse: %s", name, op.Method, op.Path, excused[name]))
		case !called[name] && excused[name] == "":
			lines = append(lines, fmt.Sprintf("%s (%s %s) is sent by embyfin's code, and no live test reached it", name, op.Method, op.Path))
		}
	}
	for _, name := range slices.Sorted(maps.Keys(excused)) {
		if ops[name] == nil {
			lines = append(lines, fmt.Sprintf("the excuse for %s names nothing embyfin's code sends", name))
		}
	}

	return lines
}
