GOCMD=go
GORUN=$(GOCMD) run
GOLINT_CONTAINER_CMD=docker run -t --rm -v $(shell pwd):/app -w /app golangci/golangci-lint:latest
CURRENT_BRANCH=$(shell git rev-parse --abbrev-ref HEAD)

.PHONY: all
all: run

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