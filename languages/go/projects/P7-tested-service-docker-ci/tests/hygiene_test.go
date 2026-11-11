package tests

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// These tests read the packaging files the learner writes. Docker itself is
// never run.

func readFile(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile("../starter/" + rel)
	if err != nil {
		t.Fatalf("missing %s: %v (create it in starter/)", rel, err)
	}
	return string(data)
}

type instruction struct{ cmd, args string }

type stage struct {
	image string
	name  string
	body  []instruction
}

// parseDockerfile joins line continuations, drops comments and groups the
// instructions per build stage.
func parseDockerfile(t *testing.T, text string) []stage {
	t.Helper()
	text = strings.ReplaceAll(text, "\\\n", " ")
	var stages []stage
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cmd, args, _ := strings.Cut(line, " ")
		cmd = strings.ToUpper(cmd)
		if cmd == "FROM" {
			fields := strings.Fields(args)
			var flagless []string
			for _, f := range fields {
				if !strings.HasPrefix(f, "--") {
					flagless = append(flagless, f)
				}
			}
			s := stage{image: flagless[0]}
			if len(flagless) == 3 && strings.EqualFold(flagless[1], "AS") {
				s.name = flagless[2]
			}
			stages = append(stages, s)
			continue
		}
		if len(stages) == 0 {
			continue
		}
		last := &stages[len(stages)-1]
		last.body = append(last.body, instruction{cmd, strings.TrimSpace(args)})
	}
	return stages
}

func dockerStages(t *testing.T) []stage {
	t.Helper()
	stages := parseDockerfile(t, readFile(t, "Dockerfile"))
	if len(stages) == 0 {
		t.Fatal("Dockerfile has no FROM instruction")
	}
	return stages
}

func has(s stage, cmd string) bool {
	return slices.ContainsFunc(s.body, func(i instruction) bool { return i.cmd == cmd })
}

func TestDockerfileIsMultiStage(t *testing.T) {
	stages := dockerStages(t)
	if len(stages) < 2 {
		t.Fatalf("Dockerfile has %d FROM stage(s), want at least 2 (build + runtime)", len(stages))
	}
}

func TestDockerfileBuildsInABuildStageWithoutCgo(t *testing.T) {
	stages := dockerStages(t)
	build := stages[0]
	var buildsGo, static bool
	for _, i := range build.body {
		if i.cmd == "RUN" && strings.Contains(i.args, "go build") {
			buildsGo = true
			static = strings.Contains(i.args, "CGO_ENABLED=0")
		}
	}
	if !buildsGo {
		t.Error("the first stage must run `go build`")
	}
	if !static {
		t.Error("`go build` must set CGO_ENABLED=0 so the binary runs on a minimal image")
	}
}

func TestDockerfileCachesModuleDownloadsBeforeCopyingSources(t *testing.T) {
	build := dockerStages(t)[0]
	download, copyAll := -1, -1
	for n, i := range build.body {
		switch {
		case i.cmd == "RUN" && strings.Contains(i.args, "go mod download") && download < 0:
			download = n
		case i.cmd == "COPY" && strings.HasPrefix(i.args, ". ") && copyAll < 0:
			copyAll = n
		}
	}
	if download < 0 || copyAll < 0 || download > copyAll {
		t.Errorf("want `go mod download` (at %d) before `COPY . .` (at %d) so the layer is cached", download, copyAll)
	}
}

func TestDockerfileFinalStageIsMinimalAndCopiesTheBinary(t *testing.T) {
	stages := dockerStages(t)
	final := stages[len(stages)-1]
	if strings.HasPrefix(final.image, "golang") {
		t.Errorf("final stage uses %q; ship a minimal runtime image, not the Go toolchain", final.image)
	}
	copied := slices.ContainsFunc(final.body, func(i instruction) bool {
		return i.cmd == "COPY" && strings.Contains(i.args, "--from=")
	})
	if !copied {
		t.Error("final stage must `COPY --from=<build stage>` the compiled binary")
	}
}

func TestDockerfileRunsAsNonRoot(t *testing.T) {
	stages := dockerStages(t)
	final := stages[len(stages)-1]
	var user string
	for _, i := range final.body {
		if i.cmd == "USER" {
			user = i.args
		}
	}
	if user == "" {
		t.Fatal("final stage has no USER instruction")
	}
	name, _, _ := strings.Cut(user, ":")
	if name == "root" || name == "0" {
		t.Errorf("USER %q is root; use a non-root user", user)
	}
}

func TestDockerfileExposesThePort(t *testing.T) {
	stages := dockerStages(t)
	if !has(stages[len(stages)-1], "EXPOSE") {
		t.Error("final stage must EXPOSE the service port")
	}
}

func TestDockerfileHasAHealthcheckOrDocumentedProbe(t *testing.T) {
	text := readFile(t, "Dockerfile")
	stages := dockerStages(t)
	probeDocumented := strings.Contains(text, "/healthz") || strings.Contains(text, "/readyz")
	if !has(stages[len(stages)-1], "HEALTHCHECK") && !probeDocumented {
		t.Error("add a HEALTHCHECK, or document the /healthz probe in a comment")
	}
}

func TestDockerignoreExcludesGitHistoryAndBuildOutput(t *testing.T) {
	lines := strings.Fields(readFile(t, ".dockerignore"))
	for _, want := range []string{".git", "bin"} {
		found := slices.ContainsFunc(lines, func(l string) bool {
			return strings.Trim(l, "/*") == want
		})
		if !found {
			t.Errorf(".dockerignore must list %q", want)
		}
	}
}

type workflow struct {
	On   any `yaml:"on"`
	Jobs map[string]struct {
		Steps []struct {
			Uses string         `yaml:"uses"`
			Run  string         `yaml:"run"`
			With map[string]any `yaml:"with"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

func loadWorkflow(t *testing.T) workflow {
	t.Helper()
	var wf workflow
	if err := yaml.Unmarshal([]byte(readFile(t, ".github/workflows/ci.yml")), &wf); err != nil {
		t.Fatalf("ci.yml is not valid YAML: %v", err)
	}
	if len(wf.Jobs) == 0 {
		t.Fatal("ci.yml defines no jobs")
	}
	return wf
}

func triggers(on any) []string {
	switch v := on.(type) {
	case string:
		return []string{v}
	case []any:
		var out []string
		for _, e := range v {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case map[string]any:
		var out []string
		for k := range v {
			out = append(out, k)
		}
		return out
	}
	return nil
}

func TestCITriggersOnPushAndPullRequest(t *testing.T) {
	got := triggers(loadWorkflow(t).On)
	for _, want := range []string{"push", "pull_request"} {
		if !slices.Contains(got, want) {
			t.Errorf("ci.yml triggers = %v, want %q included", got, want)
		}
	}
}

func TestCIRunsBuildFormatLintAndTests(t *testing.T) {
	var runs []string
	for _, job := range loadWorkflow(t).Jobs {
		for _, s := range job.Steps {
			runs = append(runs, s.Run)
		}
	}
	all := strings.Join(runs, "\n")
	for name, needle := range map[string]string{
		"build":        "go build",
		"format check": "gofmt -l",
		"lint":         "go vet",
		"test":         "go test",
	} {
		if !strings.Contains(all, needle) {
			t.Errorf("ci.yml has no %s step (expected a run command containing %q)", name, needle)
		}
	}
}

func TestCIUsesThePinnedToolchain(t *testing.T) {
	var pinned bool
	for _, job := range loadWorkflow(t).Jobs {
		for _, s := range job.Steps {
			if !strings.HasPrefix(s.Uses, "actions/setup-go@") {
				continue
			}
			file, _ := s.With["go-version-file"].(string)
			version := fmt.Sprint(s.With["go-version"])
			pinned = strings.HasSuffix(file, "go.mod") || strings.HasPrefix(version, "1.27")
		}
	}
	if !pinned {
		t.Error("ci.yml must use actions/setup-go with go-version-file pointing at go.mod (or go-version: 1.27.x)")
	}
}
