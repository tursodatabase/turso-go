SHELL := /usr/bin/env bash

BUILD_TYPE  ?= release

.PHONY: build test

build: clean
	@chmod +x ./build_lib.sh
	./build_lib.sh $(BUILD_TYPE)

test:
	TURSO_GO_NOCACHE=1 go test -v ./...

cache-print:
	scripts/cache.sh print

cache-clean:
	scripts/cache.sh clean

merge-pr:
ifndef PR
	$(error PR is required. Usage: make merge-pr PR=123)
endif
	@echo "Setting up environment for PR merge..."
	@if [ -z "$(GITHUB_REPOSITORY)" ]; then \
		REPO=$$(git remote get-url origin | sed -E 's|.*github\.com[:/]([^/]+/[^/]+?)(\.git)?$$|\1|'); \
		if [ -z "$$REPO" ]; then \
			echo "Error: Could not detect repository from git remote"; \
			exit 1; \
		fi; \
		export GITHUB_REPOSITORY="$$REPO"; \
	else \
		export GITHUB_REPOSITORY="$(GITHUB_REPOSITORY)"; \
	fi; \
	echo "Repository: $$REPO"; \
	AUTH=$$(gh auth status); \
	if [ -z "$$AUTH" ]; then \
		echo "auth: $$AUTH"; \
		echo "GitHub CLI not authenticated. Starting login process..."; \
		gh auth login --scopes repo,workflow; \
	else \
		if ! echo "$$AUTH" | grep -q "workflow"; then \
			echo "Warning: 'workflow' scope not detected. You may need to re-authenticate if merging PRs with workflow changes."; \
			echo "Run: gh auth refresh -s repo,workflow"; \
		fi; \
	fi; \
	if [ "$(LOCAL)" = "1" ]; then \
	    echo "merging PR #$(PR) locally"; \
		uv run scripts/merge-pr.py $(PR) --local; \
	else \
	    echo "merging PR #$(PR) on GitHub"; \
		uv run scripts/merge-pr.py $(PR); \
	fi

.PHONY: merge-pr
