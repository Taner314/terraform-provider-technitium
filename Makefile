default: fmt lint install generate

build:
	go build -v ./...

install: build
	go install -v ./...

lint:
	docker run --rm -v $$(pwd):/app -w /app golangci/golangci-lint:v2.15.0 golangci-lint run

## Generate docs
generate:
	cd tools; go generate ./...

validate-examples:
	./scripts/validate-examples.sh

fmt:
	gofmt -s -w -e .

test:
	go test -v -cover -timeout=120s -parallel=10 ./...

testacc:
	TF_ACC=1 go test -v -cover -timeout 120m ./...

.PHONY: fmt lint test testacc build install docs validate-examples
