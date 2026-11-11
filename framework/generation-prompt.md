# Generation Prompt

Use this prompt to generate one module for one language. Replace the placeholders. Run once per module, then verify against the Definition of Done in [README.md](./README.md).

---

## Prompt

You are an experienced <LANGUAGE> backend engineer and technical educator.

**Task:** Create learning module `<NN-topic-folder>` for <LANGUAGE> under `languages/<lang>/modules/<NN-topic-folder>/`.

**Inputs (read first):**
1. `framework/README.md`: principles, layout, definition of done
2. `framework/module.template.md`: required structure of the lesson README
3. `languages/<lang>/PROFILE.md`: toolchain, library choices, conventions, teaching notes
4. `backend-core-topics.md`: scope for topic `<NN>` (do not exceed it)

**Requirements:**
- Follow `module.template.md` exactly; keep all section headings.
- Use only the libraries named in PROFILE.md for this topic. If something else is truly needed, stop and say so.
- Teach idiomatic <LANGUAGE>. Do not port idioms from other languages.
- Create 3-5 runnable examples in `examples/`, simplest first; the final one uses the Task Tracker domain.
- Create 3 exercises in `exercises/` (easy, medium, stretch) with starter code and tests that fail until solved; put reference solutions in `solutions/`.
- Pitfalls must be specific to <LANGUAGE> and this topic.
- Comments in code only where they clarify non-obvious behavior.
- Keep the lesson concise; prefer code and tables over long prose.
- Stay in scope: HTTP only; no gRPC, GraphQL, message brokers, or advanced distributed-systems topics.

**Verification (perform before finishing):**
- Run the format, lint, and test commands from PROFILE.md.
- Confirm every example runs and prints the documented output.
- Confirm exercise tests fail against the starter and pass against the solution.

**Output:** the files created, plus a short list of any assumptions or deviations from PROFILE.md.

---

## Companion Prompts

### A. Create the language profile
> Fill in `framework/language-profile.template.md` for <LANGUAGE> targeting version <X>. Prefer the standard library; justify each third-party library with one sentence. Output to `languages/<lang>/PROFILE.md`. List open questions instead of guessing.

### B. Create the language learning path
> Using `languages/<lang>/PROFILE.md` and the generated modules, write `languages/<lang>/README.md`: prerequisites, setup instructions, ordered module list with time estimates, a progress checklist, and the list of capstone projects.

### C. Create a capstone project
> Create project `<PN-name>` for <LANGUAGE> from `backend-core-topics.md` (Suggested Hands-on Projects). Provide: README with requirements and acceptance criteria, starter skeleton, an automated test suite (black-box over HTTP where possible), and a reference solution. List which modules it exercises.

### D. Review a generated module
> Review `languages/<lang>/modules/<NN-topic-folder>/` against the Definition of Done in `framework/README.md`. Report violations as a checklist with file references. Flag non-idiomatic code, out-of-scope content, untested examples, and generic pitfalls.
