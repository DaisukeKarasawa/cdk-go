# Lambda 開発

## 概要

ブログ API のハンドラ（Go）を作成します。

**目的**: API Gatewayからのイベントを受け取り、ルーティングできる関数の最低限の土台を作成

**リスク**: Lambda 用ビルド設定の不足

## 依存関係の追加

```bash
# Go Lambda ランタイム依存
go get github.com/aws/aws-lambda-go@latest
```

## ディレクトリ構成

Lambda関数用のディレクトリを作成：

```bash
mkdir -p lambda/cmd/blog
```

## Lambda 実装

`lambda/cmd/blog/main.go` を作成：

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type Post struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

func handleRequest(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	path := req.Path
	method := req.HTTPMethod

	// 簡易ルーティング
	if method == http.MethodGet && path == "/posts" {
		// 本来は S3 から一覧を構築
		posts := []Post{{ID: 1, Title: "Hello", Content: "Hello from LocalStack"}}
		b, _ := json.Marshal(posts)
		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Body:       string(b),
			Headers:    map[string]string{"Content-Type": "application/json"},
		}, nil
	}

	if method == http.MethodGet && strings.HasPrefix(path, "/posts/") {
		idStr := strings.TrimPrefix(path, "/posts/")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 400,
				Body:       "invalid id: must be a number",
			}, nil
		}
		// 本来は S3 の `posts/{id}.json` を取得して返す
		content := fmt.Sprintf("# Post %d\n\nThis is a mock article.", id)
		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Body:       content,
			Headers:    map[string]string{"Content-Type": "text/markdown; charset=utf-8"},
		}, nil
	}

	return events.APIGatewayProxyResponse{StatusCode: 404, Body: "not found"}, nil
}

func main() {
	_ = os.Setenv("AWS_SDK_LOAD_CONFIG", "1")
	lambda.Start(handleRequest)
}
```

## ビルド方法

Lambda用のバイナリをビルド：

```bash
# Linux/amd64 用にクロスコンパイル
mkdir -p dist/blog
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/blog/bootstrap lambda/cmd/blog

# ZIP化（CDKデプロイ用）
( cd dist/blog && zip -j ../blog.zip bootstrap )
```

## 実装のポイント

### 1. エントリポイント

- Lambda Go ランタイムでは実行ファイル名を `bootstrap` にする必要があります
- `PROVIDED_AL2` ランタイムを使用

### 2. ルーティング

- API Gateway の `APIGatewayProxyRequest` から `Path` と `HTTPMethod` を取得
- 簡易的なパスマッチングでエンドポイントを分岐

### 3. レスポンス形式

- `APIGatewayProxyResponse` でHTTPステータスコード、ボディ、ヘッダーを返却
- JSON レスポンスには適切な `Content-Type` を設定

## 最小実装の制限

この実装は動作確認用の雛形です：

- **GET /posts**: 固定のモックデータを返却
- **GET /posts/{id}**: 固定のMarkdownテキストを返却
- **S3連携なし**: 実際のデータ永続化は行いません

実運用では[CRUD対応Lambda](../reference/crud-lambda.md)に置き換えてください。

## 次のステップ

Lambda関数の実装が完了したら、[CDK スタック](./05-cdk-stack.md)でインフラ定義に進んでください。
