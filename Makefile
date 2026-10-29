## Generate Go code dari semua file .proto
proto:
	cd services && protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		-I . \
		shared/proto/auth/auth.proto
	cd services && protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		-I . \
		shared/proto/link/link.proto
	cd services && protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		-I . \
		shared/proto/analytics/analytics.proto
	cd services && protoc --go_out=. --go_opt=paths=source_relative \
		-I . \
		shared/proto/events/link_events.proto

## Unit test semua service (integration test otomatis di-skip tanpa env)
test:
	cd services && go test ./...

## Integration test Kafka + Postgres (butuh: docker compose up -d postgres kafka)
## Contoh: make test-integration TEST_DSN="postgres://user:pass@localhost:5432/linkforge?sslmode=disable"
## Mencakup repository Auth/Link/Analytics, consumer restart, dan EnsureTopic.
KAFKA_TEST_BROKERS ?= localhost:9092
test-integration:
	cd services && KAFKA_TEST_BROKERS=$(KAFKA_TEST_BROKERS) TEST_DSN="$(TEST_DSN)" ANALYTICS_TEST_DSN="$(TEST_DSN)" go test -count=1 ./...

## Coverage lapisan inti per service; gagal kalau ada yang < 70% (COVERAGE_MIN=... untuk mengubah).
## Butuh TEST_DSN supaya test repository ikut jalan.
cover:
	TEST_DSN="$(TEST_DSN)" bash scripts/coverage.sh

.PHONY: proto test test-integration cover
