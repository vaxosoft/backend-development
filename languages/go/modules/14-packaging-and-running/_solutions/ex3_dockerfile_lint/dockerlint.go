// Package dockerlint is exercise 3: check a Dockerfile against production rules, as text.
package dockerlint

import (
	"slices"
	"sort"
	"strings"
)

// Finding is one violated rule.
type Finding struct {
	Rule    string
	Line    int // 1-based line of the offending instruction, 0 for whole-file findings
	Message string
}

type instruction struct {
	Cmd  string // upper case
	Args string
	Line int
}

type stage struct {
	From         instruction
	Instructions []instruction // everything after FROM
}

// Lint returns the findings sorted by line, then rule. A clean Dockerfile yields nil.
func Lint(src string) []Finding {
	stages := parse(src)
	var out []Finding
	if len(stages) < 2 {
		out = append(out, Finding{"multi-stage", 0, "use a build stage and a separate minimal runtime stage"})
	}
	for _, s := range stages {
		if f, ok := checkBase(s.From); ok {
			out = append(out, f)
		}
		out = append(out, checkDepsBeforeSource(s)...)
	}
	if len(stages) > 0 {
		out = append(out, checkFinal(stages)...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].Rule < out[j].Rule
	})
	return out
}

// parse joins continuation lines, drops comments and blank lines, and groups instructions by stage.
func parse(src string) []stage {
	var stages []stage
	var pending strings.Builder
	start := 0
	for i, raw := range strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if pending.Len() == 0 && (line == "" || strings.HasPrefix(line, "#")) {
			continue
		}
		if pending.Len() > 0 && strings.HasPrefix(line, "#") {
			continue
		}
		if pending.Len() == 0 {
			start = i + 1
		}
		if strings.HasSuffix(line, "\\") {
			pending.WriteString(strings.TrimSuffix(line, "\\") + " ")
			continue
		}
		pending.WriteString(line)
		cmd, args, _ := strings.Cut(strings.TrimSpace(pending.String()), " ")
		pending.Reset()
		in := instruction{strings.ToUpper(cmd), strings.TrimSpace(args), start}
		switch {
		case in.Cmd == "FROM":
			stages = append(stages, stage{From: in})
		case len(stages) > 0:
			stages[len(stages)-1].Instructions = append(stages[len(stages)-1].Instructions, in)
		}
	}
	return stages
}

// image returns the image reference of a FROM instruction (skipping --flags) and the stage alias.
func image(from instruction) (ref, alias string) {
	var words []string
	for _, w := range strings.Fields(from.Args) {
		if !strings.HasPrefix(w, "--") {
			words = append(words, w)
		}
	}
	if len(words) == 0 {
		return "", ""
	}
	if len(words) >= 3 && strings.EqualFold(words[1], "AS") {
		alias = words[2]
	}
	return words[0], alias
}

func checkBase(from instruction) (Finding, bool) {
	ref, _ := image(from)
	if ref == "" || ref == "scratch" || strings.Contains(ref, "$") || strings.Contains(ref, "@sha256:") {
		return Finding{}, false
	}
	name := ref[strings.LastIndex(ref, "/")+1:] // a registry port such as host:5000/img is not a tag
	_, tag, hasTag := strings.Cut(name, ":")
	if !hasTag || tag == "latest" {
		return Finding{"pinned-base", from.Line, "pin the base image to a version tag or digest: " + ref}, true
	}
	return Finding{}, false
}

func checkDepsBeforeSource(s stage) []Finding {
	sawModFiles := false
	for _, in := range s.Instructions {
		if in.Cmd != "COPY" && in.Cmd != "ADD" {
			continue
		}
		args := strings.Fields(in.Args)
		var paths []string
		fromOtherStage := false
		for i := 0; i < len(args); i++ {
			switch a := args[i]; {
			case a == "--from" && i+1 < len(args):
				fromOtherStage = true
				i++
			case strings.HasPrefix(a, "--from="):
				fromOtherStage = true
			case !strings.HasPrefix(a, "--"):
				paths = append(paths, a)
			}
		}
		if fromOtherStage || len(paths) < 2 {
			continue
		}
		for _, p := range paths[:len(paths)-1] {
			if strings.Contains(p, "go.mod") || strings.Contains(p, "go.sum") {
				sawModFiles = true
			}
		}
		if slices.Contains(paths[:len(paths)-1], ".") || slices.Contains(paths[:len(paths)-1], "./") {
			if !sawModFiles && stageBuildsGo(s) {
				return []Finding{{"deps-before-source", in.Line, "copy go.mod and go.sum and download modules before copying the source"}}
			}
			return nil
		}
	}
	return nil
}

func stageBuildsGo(s stage) bool {
	for _, in := range s.Instructions {
		if in.Cmd == "RUN" && strings.Contains(in.Args, "go build") {
			return true
		}
	}
	return false
}

func checkFinal(stages []stage) []Finding {
	final := stages[len(stages)-1]
	var out []Finding
	user, userLine, expose, healthcheck := "", 0, false, false
	for _, in := range final.Instructions {
		switch in.Cmd {
		case "USER":
			if f := strings.Fields(in.Args); len(f) > 0 {
				user, userLine = f[0], in.Line
			}
		case "EXPOSE":
			expose = true
		case "HEALTHCHECK":
			healthcheck = !strings.EqualFold(strings.TrimSpace(in.Args), "NONE")
		}
	}
	name, _, _ := strings.Cut(user, ":")
	if user == "" || name == "root" || name == "0" {
		line := final.From.Line
		if userLine > 0 {
			line = userLine
		}
		out = append(out, Finding{"non-root", line, "the final stage must run as a non-root USER"})
	}
	if !expose {
		out = append(out, Finding{"expose", final.From.Line, "document the listening port with EXPOSE"})
	}
	if !healthcheck {
		out = append(out, Finding{"healthcheck", final.From.Line, "add a HEALTHCHECK (or rely on an orchestrator probe and document it)"})
	}

	ref, _ := image(final.From)
	if ref == "scratch" || strings.Contains(ref, "distroless/static") {
		cgoOff := false
		for _, s := range stages {
			for _, in := range s.Instructions {
				if (in.Cmd == "RUN" || in.Cmd == "ENV") && strings.Contains(in.Args, "CGO_ENABLED=0") {
					cgoOff = true
				}
			}
		}
		if !cgoOff {
			out = append(out, Finding{"static-binary", final.From.Line, "a scratch/static runtime needs a binary built with CGO_ENABLED=0"})
		}
	}
	return out
}
