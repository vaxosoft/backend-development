package dockerlint

import (
	"os"
	"slices"
	"testing"
)

const good = `# syntax=docker/dockerfile:1
FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/app ./cmd/app

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
USER nonroot:nonroot
EXPOSE 8080
HEALTHCHECK CMD ["/app", "-healthcheck"]
ENTRYPOINT ["/app"]
`

func rules(fs []Finding) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Rule)
	}
	slices.Sort(out)
	return out
}

func TestGoodDockerfileIsClean(t *testing.T) {
	if got := Lint(good); len(got) != 0 {
		t.Fatalf("unexpected findings: %+v", got)
	}
}

func TestTheModuleExampleDockerfileIsClean(t *testing.T) {
	src, err := os.ReadFile("../../examples/04_service/Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	if got := Lint(string(src)); len(got) != 0 {
		t.Fatalf("the example Dockerfile violates its own rules: %+v", got)
	}
}

func TestEverythingWrong(t *testing.T) {
	bad := `FROM golang
COPY . .
RUN go build -o /app .
USER root
CMD ["/app"]
`
	got := Lint(bad)
	want := []string{"deps-before-source", "expose", "healthcheck", "multi-stage", "non-root", "pinned-base"}
	if !slices.Equal(rules(got), want) {
		t.Fatalf("rules = %v, want %v", rules(got), want)
	}
	lines := map[string]int{}
	for _, f := range got {
		lines[f.Rule] = f.Line
		if f.Message == "" {
			t.Errorf("%s has no message", f.Rule)
		}
	}
	for rule, line := range map[string]int{"pinned-base": 1, "deps-before-source": 2, "non-root": 4, "multi-stage": 0, "expose": 1} {
		if lines[rule] != line {
			t.Errorf("%s reported on line %d, want %d", rule, lines[rule], line)
		}
	}
}

func TestFindingsAreSorted(t *testing.T) {
	got := Lint("FROM golang:latest\nCOPY . .\nRUN go build .\n")
	for i := 1; i < len(got); i++ {
		a, b := got[i-1], got[i]
		if a.Line > b.Line || (a.Line == b.Line && a.Rule > b.Rule) {
			t.Fatalf("not sorted: %+v", got)
		}
	}
	if len(got) < 3 {
		t.Fatalf("expected several findings, got %+v", got)
	}
}

func TestPinnedBase(t *testing.T) {
	tests := []struct {
		from    string
		finding bool
	}{
		{"FROM golang:1.27 AS build", false},
		{"FROM golang AS build", true},
		{"FROM golang:latest AS build", true},
		{"FROM --platform=linux/amd64 golang:1.27 AS build", false},
		{"FROM --platform=linux/amd64 golang AS build", true},
		{"FROM registry.local:5000/team/golang AS build", true},
		{"FROM registry.local:5000/team/golang:1.27 AS build", false},
		{"FROM golang@sha256:0123456789abcdef AS build", false},
		{"FROM ${BASE} AS build", false},
		{"FROM scratch AS build", false},
		{"from golang:1.27 as build", false},
	}
	for _, tc := range tests {
		t.Run(tc.from, func(t *testing.T) {
			src := tc.from + "\n" + good[len("# syntax=docker/dockerfile:1\nFROM golang:1.27 AS build\n"):]
			has := slices.Contains(rules(Lint(src)), "pinned-base")
			if has != tc.finding {
				t.Fatalf("pinned-base finding = %v, want %v", has, tc.finding)
			}
		})
	}
}

func TestPinnedBaseAppliesToEveryStage(t *testing.T) {
	src := "FROM golang:1.27 AS build\nCOPY go.mod ./\nFROM alpine\nUSER app\nEXPOSE 80\nHEALTHCHECK CMD true\n"
	var got []Finding
	for _, f := range Lint(src) {
		if f.Rule == "pinned-base" {
			got = append(got, f)
		}
	}
	if len(got) != 1 || got[0].Line != 3 {
		t.Fatalf("findings = %+v, want one pinned-base on line 3", got)
	}
}

func TestNonRoot(t *testing.T) {
	tests := []struct {
		name string
		tail string
		want bool
	}{
		{"named user", "USER app\n", false},
		{"uid", "USER 10001:10001\n", false},
		{"root", "USER root\n", true},
		{"root with group", "USER root:root\n", true},
		{"zero", "USER 0\n", true},
		{"zero with group", "USER 0:0\n", true},
		{"missing", "", true},
		{"last user wins", "USER app\nUSER root\n", true},
		{"root then app", "USER root\nUSER app\n", false},
		{"lower case", "user app\n", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := "FROM golang:1.27 AS build\nUSER root\nFROM alpine:3.20\nEXPOSE 80\nHEALTHCHECK CMD true\n" + tc.tail
			has := slices.Contains(rules(Lint(src)), "non-root")
			if has != tc.want {
				t.Fatalf("non-root finding = %v, want %v", has, tc.want)
			}
		})
	}
}

func TestOnlyTheFinalStageNeedsRuntimeSettings(t *testing.T) {
	// The build stage has no USER, EXPOSE or HEALTHCHECK, and that is fine.
	src := "FROM golang:1.27 AS build\nRUN true\nFROM alpine:3.20\nUSER app\nEXPOSE 80\nHEALTHCHECK CMD true\n"
	if got := Lint(src); len(got) != 0 {
		t.Fatalf("unexpected findings: %+v", got)
	}
}

func TestHealthcheckNoneDoesNotCount(t *testing.T) {
	src := "FROM golang:1.27 AS build\nFROM alpine:3.20\nUSER app\nEXPOSE 80\nHEALTHCHECK NONE\n"
	if got := rules(Lint(src)); !slices.Equal(got, []string{"healthcheck"}) {
		t.Fatalf("rules = %v", got)
	}
}

func TestContinuationsAndComments(t *testing.T) {
	src := "FROM golang:1.27 AS build\n" +
		"COPY go.mod \\\n    go.sum ./\n" +
		"COPY . .\n" +
		"RUN CGO_ENABLED=0 \\\n    go build \\\n    # comment inside a continued instruction\n    -o /app .\n" +
		"FROM scratch\n" +
		"# USER root is only a comment\n" +
		"USER 1000\n" +
		"EXPOSE 80\n" +
		"HEALTHCHECK \\\n  CMD [\"/app\"]\n"
	if got := Lint(src); len(got) != 0 {
		t.Fatalf("unexpected findings: %+v", got)
	}
}

func TestDepsBeforeSource(t *testing.T) {
	tail := "RUN CGO_ENABLED=0 go build -o /app .\nFROM scratch\nUSER 1000\nEXPOSE 80\nHEALTHCHECK CMD [\"/app\"]\n"
	tests := []struct {
		name string
		head string
		want int // line of the finding, 0 for none
	}{
		{"modules first", "FROM golang:1.27 AS build\nCOPY go.mod go.sum ./\nCOPY . .\n", 0},
		{"source first", "FROM golang:1.27 AS build\nCOPY . .\n", 2},
		{"source first, ./", "FROM golang:1.27 AS build\nWORKDIR /src\nCOPY ./ /src/\n", 3},
		{"only some files", "FROM golang:1.27 AS build\nCOPY cmd ./cmd\nCOPY go.mod ./\nCOPY . .\n", 0},
		{"copy --from is not the context", "FROM golang:1.27 AS base\nFROM golang:1.27 AS build\nCOPY --from=base /x /x\nCOPY go.mod ./\nCOPY . .\n", 0},
		{"separate --from value", "FROM golang:1.27 AS base\nFROM golang:1.27 AS build\nCOPY --from base . .\nCOPY go.mod ./\nCOPY . .\n", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Lint(tc.head + tail)
			line := 0
			for _, f := range got {
				if f.Rule == "deps-before-source" {
					line = f.Line
				}
			}
			if line != tc.want {
				t.Fatalf("deps-before-source line = %d, want %d (all: %+v)", line, tc.want, got)
			}
		})
	}
	// A stage that never builds Go code has no module cache to protect.
	src := "FROM node:22 AS web\nCOPY . .\nRUN npm ci\nFROM nginx:1.27\nUSER nginx\nEXPOSE 80\nHEALTHCHECK CMD true\n"
	if got := Lint(src); len(got) != 0 {
		t.Fatalf("unexpected findings: %+v", got)
	}
}

func TestStaticBinaryNeedsCgoDisabled(t *testing.T) {
	build := func(run, base string) string {
		return "FROM golang:1.27 AS build\nCOPY go.mod ./\n" + run + "\nFROM " + base + "\nUSER 1000\nEXPOSE 80\nHEALTHCHECK CMD [\"/app\"]\n"
	}
	tests := []struct {
		name, run, base string
		want            bool
	}{
		{"scratch without flag", "RUN go build -o /app .", "scratch", true},
		{"scratch with flag", "RUN CGO_ENABLED=0 go build -o /app .", "scratch", false},
		{"env instruction", "ENV CGO_ENABLED=0\nRUN go build -o /app .", "scratch", false},
		{"distroless static without flag", "RUN go build -o /app .", "gcr.io/distroless/static-debian12:nonroot", true},
		{"glibc base does not need it", "RUN go build -o /app .", "debian:12-slim", false},
		{"distroless base (cgo ok)", "RUN go build -o /app .", "gcr.io/distroless/base-debian12:nonroot", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			has := slices.Contains(rules(Lint(build(tc.run, tc.base))), "static-binary")
			if has != tc.want {
				t.Fatalf("static-binary finding = %v, want %v", has, tc.want)
			}
		})
	}
}

func TestEmptyAndWindowsLineEndings(t *testing.T) {
	if got := rules(Lint("")); !slices.Equal(got, []string{"multi-stage"}) {
		t.Fatalf("empty file: %v", got)
	}
	crlf := ""
	for _, c := range good {
		if c == '\n' {
			crlf += "\r"
		}
		crlf += string(c)
	}
	if got := Lint(crlf); len(got) != 0 {
		t.Fatalf("CRLF input: %+v", got)
	}
}
