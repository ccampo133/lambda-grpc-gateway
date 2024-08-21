buf-dep-update:
	buf dep update

generate:
	rm -rf gen
	rm -rf spec
	buf generate
	go generate ./...

build-lambda-zip:
	rm -f lambda.zip
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -o bootstrap cmd/lambda/main.go
	zip lambda.zip bootstrap
