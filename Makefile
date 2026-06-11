# Build
go build ./...

# Test (when added)
go test ./...

# Run
export DEEPSEEK_API_KEY="sk-..."
go run ./cmd/gateway -config config/gateway.yaml
