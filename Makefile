.PHONY: build

build: 
	go build -tags=go_json,nomsgpack -o client-nest .

run: build
	./client-nest

run-staging: build
	./client-nest --env=staging

run-production: build
	./client-nest --env=production