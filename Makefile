SHELL := /bin/bash

# 開発環境のセットアップ
setup-dev:
	@echo "🚀 開発環境をセットアップしています..."
	docker compose up -d go-dev
	docker compose exec go-dev go mod download
	@echo "✅ 開発環境のセットアップが完了しました"

# Docker環境でのビルド
build-docker:
	@echo "🔨 Dockerコンテナ内でビルドしています..."
	docker compose exec go-dev sh -c "mkdir -p dist/blog && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/blog/bootstrap ./lambda/cmd/blog && cd dist/blog && zip -j ../blog.zip bootstrap"
	@echo "✅ ビルドが完了しました"

# Docker環境でのテスト
test-docker:
	@echo "🧪 Dockerコンテナ内でテストを実行しています..."
	docker compose exec go-dev go test ./...
	@echo "✅ テストが完了しました"

# 開発環境のクリーンアップ
clean-dev:
	@echo "🧹 開発環境をクリーンアップしています..."
	docker compose down
	docker volume rm cdk-go_go-mod-cache 2>/dev/null || true
	@echo "✅ クリーンアップが完了しました"

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
