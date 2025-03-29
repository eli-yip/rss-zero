current_branch := shell("git branch --show-current")

# Build all
build: build-backend build-frontend
    mv server/server-app .
    rm -rf dist
    mv webapp/dist .
    @echo "Build successful"
    ./server-app --config=server/config.toml

clean:
    rm -rf server-app dist

# Lint the backend and frontend code
lint: lint-backend lint-frontend

update: update-backend update-frontend

[working-directory('server')]
cli:
    go run ./cmd/cli

[working-directory('server')]
tidy:
    go mod tidy

[working-directory('server')]
build-backend:
    go build -o server-app ./cmd/server

[working-directory('server')]
lint-backend:
    golangci-lint run -v --timeout 5m

[working-directory('server')]
update-backend:
    go get -u ./...
    go mod tidy

[working-directory('server')]
backend:
    go run ./cmd/server --config=config.toml

[working-directory('webapp')]
format:
    bunx prettier --write .

[working-directory('webapp')]
frontend:
    bun run dev

[working-directory('webapp')]
update-frontend:
    bun outdated; bun update

[working-directory('webapp')]
lint-frontend:
    bunx eslint .

[working-directory('webapp')]
format-frontend:
    bunx prettier --write .

[working-directory('webapp')]
build-frontend:
    bun run build

commit:
    git add -A
    git commit -v

push:
    git push -u origin {{ current_branch }}
    git push --tags --no-verify

fpush:
    git push -u origin {{ current_branch }} --force-with-lease --tags

conclude:
    git diff --stat @{0.day.ago.midnight} | sort -k3nr

tpush: && push
    git-bump

dtag +tags:
    #!/usr/bin/env bash
    for tag in {{ tags }}; do
      git tag -d "${tag}" && \
      git push origin --delete "${tag}"
    done

@ltag:
    git tag --list --sort -v:refname -n

[working-directory('server')]
test:
    go test -v {{ invocation_directory() }}

switch:
    if [ {{ current_branch }} != "master" ]; then \
      git switch master; \
      git branch -d {{ current_branch }} || true; \
      git push origin --delete {{ current_branch }} || true; \
      git branch -dr origin/{{ current_branch }} || true; \
    fi

lol:
    git lol

amend:
    git add -A
    git commit --amend --no-edit
