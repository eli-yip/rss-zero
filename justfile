current_branch := shell("git branch --show-current")

lint:
  golangci-lint run -v --timeout 5m

update:
  go get -u ./...
  go mod tidy

run:
  go run . --config=config.toml

commit:
  git add -A
  git commit -v

push:
  git push -u origin {{current_branch}}
  git push --tags

fpush:
  git push -u origin {{current_branch}} --force-with-lease

conclude:
  git diff --stat @{0.day.ago.midnight} | sort -k3nr

tag:
  #!/usr/bin/env bash
  tag_name=$(date +"%Y%m%d.%H%M")
  git tag ${tag_name}
  echo "Created tag: ${tag_name}"

dtag +tags:
  #!/usr/bin/env bash
  for tag in {{tags}}; do
    git tag -d "${tag}" && \
    git push origin --delete "${tag}"
  done

@ltag:
  git tag | rg -v "v" | sort -r | xargs -n 5 | less

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
