#!/bin/bash
set -e

echo "Generating OpenAPI spec from Go annotations..."
cd "$(dirname "$0")/../../server"
rm -rf api/spec
~/go/bin/swag init -g cmd/server/main.go -o api/spec

# swag emits a docs.go importing github.com/swaggo/swag, which is not a module
# dependency — leaving it in the tree breaks `go vet ./...`. Only the JSON/YAML
# spec is consumed downstream.
rm -f api/spec/docs.go

echo "Generating TypeScript client..."
cd ../ui
rm -rf src/lib/api/generated
npx openapi-generator-cli generate -c openapi-generator-config.json

echo "✓ API client generated successfully"
