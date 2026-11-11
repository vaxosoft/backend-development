// Package dockerlint is exercise 3: check a Dockerfile against production rules, as text.
// Docker itself is never run; this is the kind of check a CI job or repo-hygiene test performs.
//
// Run the tests: go test ./modules/14-packaging-and-running/exercises/ex3_dockerfile_lint/
package dockerlint

// Finding is one violated rule.
type Finding struct {
	Rule    string
	Line    int // 1-based line where the offending instruction starts, 0 for whole-file findings
	Message string
}

// Lint returns the findings sorted by Line, then Rule. A clean Dockerfile yields nil.
//
// Parsing: instruction names are case-insensitive; a trailing backslash continues the
// instruction on the next line (the Line of an instruction is its first line); blank lines and
// # comments are skipped, also inside a continued instruction.
// A stage starts at each FROM; "the final stage" is the last one.
//
// Rules (Finding.Rule values and where they are reported):
//
//	multi-stage         fewer than 2 FROM instructions                          Line 0
//	pinned-base         a FROM image without a tag, or tagged :latest           Line of that FROM
//	                    (skip scratch, references containing $, and @sha256: digests;
//	                    a registry port as in host:5000/img is not a tag)
//	non-root            final stage has no USER, or its last USER is root/0     Line of that USER,
//	                    (also root:grp, 0:0)                                    else of the final FROM
//	expose              final stage has no EXPOSE                               Line of the final FROM
//	healthcheck         final stage has no HEALTHCHECK, or only HEALTHCHECK NONE   Line of the final FROM
//	deps-before-source  in a stage that runs "go build", the first COPY of the whole
//	                    context ("." or "./" as a source, without --from) comes before
//	                    any COPY of go.mod/go.sum                               Line of that COPY
//	static-binary       final image is scratch or distroless/static but no RUN or ENV
//	                    anywhere sets CGO_ENABLED=0                             Line of the final FROM
func Lint(src string) []Finding {
	return nil // TODO
}
