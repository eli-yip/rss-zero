GOCMD=go
GORUN=$(GOCMD) run
CURRENT_BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
CURRENT_DIR := $(CURDIR)

server:
	$(GOCMD) build -o server ${CURRENT_DIR}/cmd/server

cli:
	$(GOCMD) build -o cli ${CURRENT_DIR}/cmd/cli

.PHONY: serve
serve:
	$(GORUN) ${CURRENT_DIR}/cmd/server --config=${CURRENT_DIR}/config.toml

.PHONY: lint
lint:
	golangci-lint run -v --timeout 5m

.PHONY: full-lint
full-lint:
	go get -u
	go mod tidy
	golangci-lint run -v --timeout 5m

.PHONY: switch
switch:
	@if [ "$(CURRENT_BRANCH)" = "master" ]; then \
		echo "You are already on the master branch."; \
	else \
		git switch master; \
		git branch -d $(CURRENT_BRANCH) || true; \
		git branch -dr origin/$(CURRENT_BRANCH) || true; \
	fi

.PHONY: add
add:
	git add -A

.PHONY: commit
commit:
	git add -A
	git commit -v

.PHONY: push
push:
	git push -u origin $(CURRENT_BRANCH)
	git push --tags

.PHONY: fpush
fpush:
	git push -u origin $(CURRENT_BRANCH) --force-with-lease

.PHONY: update
update:
	go get -u ./...
	go mod tidy

.PHONY: conclude
conclude:
	git diff --stat @{0.day.ago.midnight} | sort -k3nr

.PHONY: list-tag
list-tag:
	git tag --sort=-v:refname