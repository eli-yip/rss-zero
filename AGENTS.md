# AGENTS.md

- Before writing a commit message, read the recent commit history. Use the Conventional Commit style and write commit messages in English.
- Unless explicitly requested, do not run the full existing project test suite. Prefer targeted tests for the files or packages changed in the current step.
- Run lint with `just lint`; do not write your own lint command.

## Project documentation workflow

Documentation lives under `docs/`. Use the following layout and naming conventions. `NO` is a zero-padded sequence number (`01`, `02`, …) that disambiguates documents created on the same date; `<topic>` is a short kebab-case slug.

- **SPECs** go in `docs/specs/`, named `YYYY-MM-DD-NO-<topic>.md`. A SPEC defines what to build and why before implementation.
- **PLANs** go in `docs/plans/`, named `YYYY-MM-DD-NO-<topic>.md`. A PLAN breaks a SPEC into concrete implementation steps.
- **LESSONs** go in `docs/lessons/`, named `YYYY-MM-DD-NO-<topic>.md`. While executing a PLAN, append experience and lessons learned as you go; once the PLAN is complete, reorganize the file into a coherent summary.
- **`docs/PROGRESS.md`** tracks current progress. Keep it up to date as work advances.

## Development workflow

Follow this process for every change:

1. **Discuss the SPEC first, then write a PLAN, then implement and record LESSONs.** Agree on the SPEC (what to build and why) before planning; break it into a PLAN before writing code; capture experience in the LESSON while implementing. Track progress in `docs/PROGRESS.md` throughout. See *Project documentation workflow* above for file layout and naming.
2. **Work on a dedicated branch with small commits.** Create a new branch off `master` named `feat-xxxx` (a short kebab-case description). Commit in small, focused steps rather than one large commit.
3. **Request review before merging.** When the work is complete, ask the author to review it. After the review is approved, squash merge the branch into `master` and delete the branch.
