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
