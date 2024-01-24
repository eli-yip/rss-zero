GOCMD=go
GORUN=$(GOCMD) run
GOLINT_CONTAINER_CMD=docker run -t --rm -w /app -v $(shell pwd):/app -v $(shell go env GOCACHE):/cache/go -e GOLANGCI_LINT_CACHE=/cache/go -v ${GOPATH}/pkg:/go/pkg -e GOCACHE=/cache/go golangci/golangci-lint:v1.55-alpine
CURRENT_BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
CURRENT_DIR := $(CURDIR)

.PHONY: all
all: run

.PHONY: server
server:
	$(GOCMD) build -o server ${CURRENT_DIR}/cmd/server

.PHONY: zsxq-crawler
zsxq-crawler:
	$(GOCMD) build -o zsxq-crawler ${CURRENT_DIR}/cmd/crawler/main.go

.PHONY: zsxq-re-fmt
zsxq-re-fmt:
	$(GOCMD) build -o zsxq-re-fmt ${CURRENT_DIR}/cmd/re-fmt/main.go

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
