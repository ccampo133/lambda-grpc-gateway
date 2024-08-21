buf-dep-update:
	buf dep update

generate:
	rm -rf gen
	rm -rf spec
	buf generate
	go generate ./...
