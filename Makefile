.PHONY: test unit embed

ifneq (,$(wildcard .env))
include .env
export
endif

test:
	go run cmd/test/main.go

unit:
	go test ./core/... -count=1 -cover

embed:
	go test ./core/openai/ -v -run TestEmbed_Integration -count=1
