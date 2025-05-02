current_branch := shell("git branch --show-current")

# 构建后端
build:
    go build -o server-app ./cmd/server

# 清理构建产物
clean:
    rm -rf server-app

# 运行后端服务
server:
    go run ./cmd/server --config=config.toml

# 运行CLI工具
cli:
    go run ./cmd/cli

# 整理Go依赖
tidy:
    go mod tidy

# 运行代码检查
lint:
    golangci-lint run -v --timeout 5m

# 更新依赖
update:
    go get -u ./...
    go mod tidy

# 运行测试
test:
    go test -v {{ invocation_directory() }}

# Git相关命令
commit:
    git add -A
    git commit -v

push:
    git push -u origin {{ current_branch }}
    git push --tags --no-verify

fpush:
    git push -u origin {{ current_branch }} --force-with-lease --tags

amend:
    git add -A
    git commit --amend --no-edit

lol:
    git lol

switch:
    if [ {{ current_branch }} != "master" ]; then \
      git switch master; \
      git branch -d {{ current_branch }} || true; \
      git push origin --delete {{ current_branch }} || true; \
      git branch -dr origin/{{ current_branch }} || true; \
    fi

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