GOCMD=go
GORUN=$(GOCMD) run
CURRENT_BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
CURRENT_DIR := $(CURDIR)

.PHONY: server
server:
	$(GOCMD) build -o server ${CURRENT_DIR}/cmd/server

.PHONY: zhihu-crawler
zhihu-crawler:
	$(GOCMD) build -o zhihu-crawler ${CURRENT_DIR}/cmd/crawler

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
	golangci-lint run -v --timeout 5m

.PHONY: full-lint
full-lint:
	go get -u
	go mod tidy
	golangci-lint run -v --timeout 5m

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
	git push -u origin $(CURRENT_BRANCH)

.PHONY: fpush
fpush:
	git push -u origin $(CURRENT_BRANCH) --force-with-lease

.PHONY: update
update:
	go get -u ./...
	go mod tidy
