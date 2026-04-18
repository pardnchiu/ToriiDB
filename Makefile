.PHONY: test unit embed

test:
	go run cmd/test/main.go

unit:
	go test ./... -count=1

embed:
	go test ./core/openai/ -v -run TestEmbed_Integration -count=1
