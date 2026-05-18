.PHONY: help packages-minor-autoupdate

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: ## Build local binaries
	bash ./scripts/compile_commands.sh

test: ## Run golang tests
	go test -race ./...

coverage-output: ## Get coverage output to cover.out
	go test ./... -coverprofile=cover.out

coverage-show-func: ## Display coverage output
	go tool cover -func cover.out

packages-minor-autoupdate:
	go mod edit -json \
		| jq ".Require \
			| map(select(.Indirect | not).Path) \
			| map(select( \
				. != \"github.com/bradfitz/gomemcache\" \
				and . != \"github.com/prometheus/prometheus\" \
			))" \
		| tr -d '\n' | tr -d '  '

.PHONY: assert-no-changed-files
assert-no-changed-files:
	@git update-index --refresh
	@git diff-index --quiet HEAD --

