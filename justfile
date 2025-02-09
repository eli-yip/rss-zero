current_branch := shell("git branch --show-current")

# Lint the backend and frontend code
lint: lint-backend lint-frontend

update: update-backend update-frontend

[working-directory: 'server']
lint-backend:
  golangci-lint run -v --timeout 5m

[working-directory: 'server']
update-backend:
  go get -u ./...
  go mod tidy

[working-directory: 'server']
backend:
  go run . --config=config.toml

[working-directory: 'webapp']
format:
  npx prettier --write .

[working-directory: 'webapp']
frontend:
  npm run dev

[working-directory: 'webapp']
update-frontend:
  npm outdated; npm update

[working-directory: 'webapp']
lint-frontend:
  npx eslint .

[working-directory: 'webapp']
format-frontend:
  npx prettier --write .

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

tpush: && push
  #!/usr/bin/env bash
  echo "Latest tags(3):"
  just ltag | head -n 3
  read -p "Input tag name: " tag_name
  if [ -z "$tag_name" ]; then
    echo "Empty tag is forbidden"
    exit 1
  fi
  git tag "$tag_name"

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
