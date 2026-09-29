SERVICES := macro-service options-service trigger-service execution-service
AGENT_MACRO_SCHEMA := .trae/skills/macro-risk-analyst/assets/macro-risk-report.schema.json
SERVICE_MACRO_SCHEMA := services/macro-service/internal/report/macro-risk-report.schema.json

.PHONY: contracts dev down fmt test test-race tidy up vet

dev:
	@exec ./scripts/dev.sh

up:
	docker compose up --build

down:
	docker compose down

contracts:
	cmp $(AGENT_MACRO_SCHEMA) $(SERVICE_MACRO_SCHEMA)

fmt:
	@for service in $(SERVICES); do \
		(cd services/$$service && gofmt -w $$(find . -name '*.go' -type f)); \
	done

tidy:
	@for service in $(SERVICES); do \
		(cd services/$$service && go mod tidy); \
	done
	go work sync

test: contracts
	@for service in $(SERVICES); do \
		echo "==> testing $$service"; \
		(cd services/$$service && go test ./...) || exit 1; \
	done

test-race: contracts
	@for service in $(SERVICES); do \
		echo "==> race testing $$service"; \
		(cd services/$$service && go test -race ./...) || exit 1; \
	done

vet:
	@for service in $(SERVICES); do \
		echo "==> vetting $$service"; \
		(cd services/$$service && go vet ./...) || exit 1; \
	done
