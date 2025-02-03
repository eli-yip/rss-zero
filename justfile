current_branch := shell("git branch --show-current")

[working-directory: 'server']
lint:
  golangci-lint run -v --timeout 5m

[working-directory: 'server']
update:
  go get -u ./...
  go mod tidy

[working-directory: 'server']
run:
  go run . --config=config.toml

[working-directory: 'webapp']
format:
  npx prettier --write .

[working-directory: 'webapp']
frontend:
  npm run dev

commit:
  git add -A
  git commit -v

push:
  git push -u origin {{current_branch}}
  git push --tags

fpush:
  git push -u origin {{current_branch}} --force-with-lease --tags

conclude:
  git diff --stat @{0.day.ago.midnight} | sort -k3nr

tag tag_name:
  git tag {{tag_name}}

dtag +tags:
  #!/usr/bin/env bash
  for tag in {{tags}}; do
    git tag -d "${tag}" && \
    git push origin --delete "${tag}"
  done

@ltag:
  git tag --list --sort -v:refname -n

test:
  go test -v {{invocation_directory()}}

switch:
  if [ {{current_branch}} != "master" ]; then \
    git switch master; \
    git branch -d {{current_branch}} || true; \
    git push origin --delete {{current_branch}} || true; \
    git branch -dr origin/{{current_branch}} || true; \
  fi

lol:
  git lol

amend:
  git add -A
  git commit --amend --no-edit
