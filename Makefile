.PHONY: setup run build migrate migrate-status migrate-rollback migrate-fresh seed migration scaffold rename test tidy fmt vet lint clean

setup: ## copy .env and download dependencies
	@test -f .env || cp .env.example .env
	go mod download

run: ## start the API server
	go run ./cmd/api

build: ## compile both binaries into ./bin
	go build -o bin/api ./cmd/api
	go build -o bin/migrate ./cmd/migrate

migrate: ## apply pending migrations
	go run ./cmd/migrate

migrate-status: ## list every migration and whether it has been applied
	go run ./cmd/migrate -status

migrate-rollback: ## reverse the last batch: make migrate-rollback [steps=2]
	go run ./cmd/migrate -rollback $(if $(steps),-steps $(steps))

migrate-fresh: ## drop all tables, migrate, seed
	go run ./cmd/migrate -fresh -seed

seed: ## run seeders only
	go run ./cmd/migrate -seed

migration: ## generate a migration: make migration name=add_status_to_posts
	@test -n "$(name)" || (echo "usage: make migration name=add_status_to_posts"; exit 1)
	go run ./cmd/make migration $(name)

scaffold: ## generate a full resource: make scaffold name=Category
	@test -n "$(name)" || (echo "usage: make scaffold name=Category"; exit 1)
	go run ./cmd/make scaffold $(name)

rename: ## rename the project: make rename module=github.com/you/shop app="Shop API"
	@test -n "$(module)$(app)" || (echo 'usage: make rename module=github.com/you/shop [app="Shop API"]'; exit 1)
	go run ./cmd/make rename $(module) $(if $(app),-app-name "$(app)")
	go mod tidy

test:
	go test ./... -race -count=1

tidy:
	go mod tidy

fmt:
	go fmt ./...

vet:
	go vet ./...

clean:
	rm -rf bin
