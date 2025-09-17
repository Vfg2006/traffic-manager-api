APP_NAME?=$(shell pwd | xargs basename)
APP_DIR=/app/${APP_NAME}
INTERACTIVE:=$(shell [ -t 0 ] && echo i || echo d)
PWD=$(shell pwd)
HTTP_PORT:=8000
DEBUG_PORT_SERVER:=8001
GOCACHE:=$(shell go env GOCACHE)
DOCKER_STAGE?=development

welcome:

.env:
	@cp .env.example .env

build-dev: welcome .env
	@docker build \
		--target ${DOCKER_STAGE} \
		-t hu/${APP_NAME} \
		.

test:
	@go test ./... -race -cover -count=1 -timeout=10m

debug-server: welcome .env build-dev ## Runs http server in debug mode
	@echo 'Running on http://localhost:$(HTTP_PORT)/healthcheck'

	@docker run -t${INTERACTIVE} --rm \
		-v ${PWD}:${APP_DIR}:delegated \
		-v ${GOCACHE}:/cache/go -e GOCACHE=/cache/go -e GOLANGCI_LINT_CACHE=/cache/go \
		-w ${APP_DIR} \
		--expose $(DEBUG_PORT_SERVER) \
		--expose 80 \
		-p $(HTTP_PORT):80 \
		--name ${APP_NAME} \
		${APP_NAME} \
		modd -f ./cmd/api/debug_modd.conf

start:
	go run cmd/api/main.go

migrate:
	@go run infrastructure/migration/script/script.go

dump-local:
	@echo "Dumping local database..."
	cd ../../Documentos/dump_database
	PGPASSWORD=root pg_dump -U postgres -h localhost -p 5436 -d traffic --no-password -F c -b -v -f ../../Documentos/dump_database/traffic.dump

restore-hml:
	@echo "Restoring local database..."
	@mkdir -p ../../Documentos/dump_database
	@PGPASSWORD=WZ29xXAPlF1lPKpnzqvgDbcPCliLH5hX PGSSLMODE=require pg_restore -U traffic_user -h dpg-cvo3d5gdl3ps73f0seag-a.virginia-postgres.render.com -p 5432 -d traffic_r5r4 --no-password -v ../../Documentos/dump_database/traffic.dump

# Mock generation commands
generate-mocks: ## Generate all repository mocks
	@echo "Generating repository mocks..."
	@mockgen -source=infrastructure/integrator/ssotica/service.go -destination=infrastructure/integrator/ssotica/mocks/mock_service.go -package=mocks
	@mockgen -source=infrastructure/repository/account.go -destination=infrastructure/repository/mocks/mock_account_repository.go -package=mocks
	@mockgen -source=infrastructure/repository/ad_insight.go -destination=infrastructure/repository/mocks/mock_ad_insight_repository.go -package=mocks
	@mockgen -source=infrastructure/repository/monthly_ad_insight.go -destination=infrastructure/repository/mocks/mock_monthly_ad_insight_repository.go -package=mocks
	@mockgen -source=infrastructure/repository/monthly_sales_insight.go -destination=infrastructure/repository/mocks/mock_monthly_sales_insight_repository.go -package=mocks
	@mockgen -source=infrastructure/repository/sales_insight.go -destination=infrastructure/repository/mocks/mock_sales_insight_repository.go -package=mocks
	@mockgen -source=infrastructure/repository/store_ranking.go -destination=infrastructure/repository/mocks/mock_store_ranking_repository.go -package=mocks
	@mockgen -source=infrastructure/repository/user.go -destination=infrastructure/repository/mocks/mock_user_repository.go -package=mocks
	@echo "All mocks generated successfully!"

	