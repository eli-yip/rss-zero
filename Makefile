GOCMD=go
GORUN=$(GOCMD) run
GOLINT_CONTAINER_CMD=docker run -t --rm -w /app -v $(shell pwd):/app -v ${GOPATH}/pkg/mod:/cache/mod  -v $(shell go env GOCACHE):/cache -e GOCACHE=/cache -e GOMODCACHE=/cache/mod -e GOLANGCI_LINT_CACHE=/cache golangci/golangci-lint:v1.55-alpine
CURRENT_BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
CURRENT_DIR := $(CURDIR)

.PHONY: server
server:
	$(GOCMD) build -o server ${CURRENT_DIR}/cmd/server

.PHONY: zhihu-crawler
zhihu-crawler:
	$(GOCMD) build -o zhihu-crawler ${CURRENT_DIR}/cmd/zhihu/crawler

.PHONY: zsxq-crawler
zsxq-crawler:
	$(GOCMD) build -o zsxq-crawler ${CURRENT_DIR}/cmd/zsxq/crawler

.PHONY: zsxq-re-fmt
zsxq-re-fmt:
	$(GOCMD) build -o zsxq-re-fmt ${CURRENT_DIR}/cmd/zsxq/re-fmt

.PHONY: zhihu-encrypt
zhihu-encrypt:
	node cmd/zhihu_encrypt/server.js

.PHONY: lint
lint:
	$(GOLINT_CONTAINER_CMD) golangci-lint run -v --timeout 5m

.PHONY: full-lint
full-lint:
	go get -u
	go mod tidy
	$(GOLINT_CONTAINER_CMD) golangci-lint run -v --timeout 5m

.PHONY: switch
switch:
	@if [ "$(CURRENT_BRANCH)" = "dev" ]; then \
		echo "You are already on the dev branch."; \
	else \
		git switch dev; \
		git branch -d $(CURRENT_BRANCH) || true; \
		git branch -dr origin/$(CURRENT_BRANCH) || true; \
	fi

.PHONY: commit
commit:
	git add -A
	git commit -v

.PHONY: push
push:
	git push -u origin $(CURRENT_BRANCH) --force-with-lease

.PHONY: update
update:
	go get -u ./...
	go mod tidy
