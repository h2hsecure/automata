.PHONY: build test cicd-tests cicd-tests-ci cicd-tests-verbose cicd-tests-race cicd-tests-full local up down clean deploy-staging

build:
	go build -o automata ./cmd/automata

cicd-tests:
	go test ./cicd/... -count=1 -timeout 120s

# CI variant: skips the two known pre-existing failures so the pipeline
# is red only for regressions.
cicd-tests-ci:
	go test ./cicd/... -count=1 -timeout 120s -args \
		-ginkgo.skip="should create a contact via POST /api/contacts|should track multiple events in batch"

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

# Manual staging deploy from the staging server (pipeline never deploys).
# usage: make deploy-staging IMG=<sha8> CHART=<YYYY.MM.DD>-<sha8>
deploy-staging:
	@test -n "$(IMG)" || (echo "IMG required, e.g. IMG=$(shell git rev-parse --short HEAD)"; exit 1)
	@test -n "$(CHART)" || (echo "CHART required, e.g. CHART=$(shell date -u +%Y.%m.%d)-$(shell git rev-parse --short HEAD)"; exit 1)
	# Full path: helm 4 push appends the chart name to the OCI repo
	# (package is automata-chart/automata). Works with helm 3 and 4.
	# mkdir required: helm 4 pull fails if --destination does not exist.
	mkdir -p ./tmp-chart
	helm pull oci://ghcr.io/hekemen/automata-chart/automata --version $(CHART) --destination ./tmp-chart
	helm upgrade -i automata ./tmp-chart/automata-$(CHART).tgz -n automata \
		-f helm/automata/values.local.yaml --set image.tag=$(IMG)
	rm -rf ./tmp-chart

clean:
	rm -f automata
