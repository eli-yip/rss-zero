current_branch := shell("git branch --show-current")

lint:
  golangci-lint run -v --timeout 5m

update:
  go get -u ./...
  go mod tidy

run:
  go run ./cmd/server --config=config.toml

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

tag tag_name:
  git tag {{tag_name}}

list-tag:
  git tag --sort=-v:refname

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