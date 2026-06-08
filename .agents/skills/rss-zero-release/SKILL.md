---
name: rss-zero-release
description: Release the rss-zero server project by calculating the next CalVer tag, pushing the Git tag, building Docker images through the project's justfile command, and pushing eliyip/rss-zero images. Use when working in /Users/yip/new_home/projects/rss-zero/server and the user asks to publish, release, tag the latest version, build and push Docker images, or follow the standard rss-zero server release flow.
---

# RSS Zero Release

Use this workflow only for `/Users/yip/new_home/projects/rss-zero/server`.

## Release Contract

- Release tag format: `YY.M.MICRO`, for example `26.6.4`.
- Ignore legacy tags when calculating the next release:
  - `v*` semantic-version tags
  - `YYYYMMDD.HHMM` timestamp tags
- Increment `MICRO` within the current year-month CalVer line. If the current month has no `YY.M.*` tag, start at `YY.M.0`.
- Docker image repository: `eliyip/rss-zero`.
- Run Docker builds through the justfile recipe, not by calling the underlying script directly.
- The build recipe pushes both:
  - `eliyip/rss-zero:<tag>`
  - `eliyip/rss-zero:latest`

## Workflow

1. Start in the project root:

   ```bash
   cd /Users/yip/new_home/projects/rss-zero/server
   ```

2. Check the working tree and current HEAD before publishing:

   ```bash
   git status --short --branch
   git show -s --format='%h %s' HEAD
   git tag --points-at HEAD
   ```

   Do not release from a dirty working tree unless the only changes are unrelated and the user explicitly accepts that state.

3. Check local and remote CalVer tag history:

   ```bash
   git tag --list | rg '^[0-9]{2}\.[0-9]{1,2}\.[0-9]+$' | sort -Vr | head -n 20
   git ls-remote --tags origin | rg 'refs/tags/[0-9]{2}\.[0-9]{1,2}\.[0-9]+$'
   ```

4. Choose the next tag.

   Use the current calendar year and month in the user's local timezone. For example, on 2026-06-08, continue the `26.6.*` series. If the latest matching tag is `26.6.3`, the next tag is `26.6.4`.

5. Create and push the tag:

   ```bash
   git tag <tag>
   git push origin <tag>
   ```

   If sandboxing blocks writes to `.git`, request approval for `git tag`. If hooks run on push, let them run and report failures.

6. Build and push Docker images through just:

   ```bash
   just build-docker --tag <tag> --push
   ```

   Do not replace this with `scripts/build-docker.sh --tag <tag> --push` unless the just recipe is broken and the user agrees.

7. Verify the result:

   ```bash
   git status --short --branch
   git tag --points-at HEAD
   docker image inspect eliyip/rss-zero:<tag>
   ```

   In the final response, include the tag, the pushed image names, and the digest from the build or image inspection.

## Failure Handling

- If the remote already has the chosen tag, stop and recalculate from remote tags before doing anything destructive.
- Do not delete or move a tag without explicit user approval.
- If Docker push fails due authentication or registry permission, report the exact image tag that failed and leave the Git tag in place.
- Do not run the full project test suite as part of release unless the user explicitly asks; this matches the project `AGENTS.md` guidance.
