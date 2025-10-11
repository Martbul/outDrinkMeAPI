.PHONY: test test-integration test-unit test-coverage

test:
	go test ./... -v

test-integration:
	go test ./tests/integration/... -v

test-unit:
	go test ./services/... ./handlers/... -v

test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out

test-watch:
	# Install: go install github.com/githubnemo/CompileDaemon@latest
	CompileDaemon -command="go test ./... -v"
