SHELL := /bin/bash

build-lambda:
	mkdir -p dist/blog
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/blog/bootstrap ./lambda/cmd/blog
	cd dist/blog && zip -j ../blog.zip bootstrap

bootstrap:
	cdklocal bootstrap aws://000000000000/ap-northeast-1

deploy: build-lambda
	cdklocal deploy --require-approval never

destroy:
	cdklocal destroy --force

synth:
	cdklocal synth

logs:
	awslocal logs tail "/aws/lambda/BlogApi" --follow
