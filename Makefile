.PHONY: build test cicd-tests cicd-tests-verbose cicd-tests-race cicd-tests-full local up down clean

build:
	go build -o automata ./cmd/automata

cicd-tests:
	go test ./cicd/... -count=1 -timeout 120s

cicd-tests-verbose:
	go test ./cicd/... -count=1 -timeout 120s -v -ginkgo.v

cicd-tests-race:
	go test ./cicd/... -count=1 -timeout 120s -race

cicd-tests-full:
	go test ./cicd/... -count=1 -timeout 120s -v -ginkgo.v -ginkgo.trace

test: cicd-tests

local:
	CONFIG_FILE=config.local.yaml go run ./cmd/automata

up:
	docker compose up --build -d

down:
	docker compose down -v

clean:
	rm -f automata
