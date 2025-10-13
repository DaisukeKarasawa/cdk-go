# CRUD対応Lambda（完全版）

## 概要

[最小実装](../getting-started/04-lambda-development.md)を、S3を読み書きする本格的なCRUDに差し替えます。

**目的**: ブログ記事をAPI経由でフルCRUD操作できるようになり、クライアントやCIから統一的にデータ管理が可能になる

**機能**: API Gatewayからのリクエストを受け、S3バケット `POSTS_BUCKET` に `posts/{id}.json` を生成/更新/削除し、取得時はJSON（一覧/1件）を返す

## 実装

`lambda/cmd/blog/main.go` を以下の実装に置き換えてください：

```go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Post struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

var (
	s3Client *s3.Client
	bucket   string
)

func init() {
	bucket = os.Getenv("POSTS_BUCKET")
	cfg, _ := config.LoadDefaultConfig(context.Background())
	s3Client = s3.NewFromConfig(cfg)
}

func handle(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	path := req.Path
	method := req.HTTPMethod

	if method == http.MethodGet && path == "/posts" {
		// 一覧: posts/*.json を読み込んで配列にして返す
		prefix := "posts/"
		out, err := s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: &bucket, Prefix: &prefix})
		if err != nil {
			return errorJSON(500, "list failed")
		}
		posts := make([]Post, 0)
		for _, obj := range out.Contents {
			key := *obj.Key
			if !strings.HasSuffix(key, ".json") {
				continue
			}
			po, err := s3Client.GetObject(ctx, &s3.GetObjectInput{Bucket: &bucket, Key: &key})
			if err != nil {
				continue
			}
			b, _ := io.ReadAll(po.Body)
			_ = po.Body.Close()
			var p Post
			if json.Unmarshal(b, &p) == nil {
				posts = append(posts, p)
			}
		}
		return jsonOK(posts), nil
	}

	if method == http.MethodGet && strings.HasPrefix(path, "/posts/") {
		idStr := strings.TrimPrefix(path, "/posts/")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return errorJSON(400, "invalid id: must be a number")
		}
		key := fmt.Sprintf("posts/%d.json", id)
		po, err := s3Client.GetObject(ctx, &s3.GetObjectInput{Bucket: &bucket, Key: &key})
		if err != nil {
			return errorJSON(404, "not found")
		}
		b, _ := io.ReadAll(po.Body)
		_ = po.Body.Close()
		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Body:       string(b),
			Headers:    map[string]string{"Content-Type": "application/json"},
		}, nil
	}

	if method == http.MethodPost && path == "/posts" {
		var p Post
		if err := json.Unmarshal([]byte(req.Body), &p); err != nil || p.ID == 0 {
			return errorJSON(400, "invalid body: require id,title,content")
		}
		key := fmt.Sprintf("posts/%d.json", p.ID)
		b, _ := json.Marshal(p)
		ct := "application/json"
		_, err := s3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      &bucket,
			Key:         &key,
			Body:        bytes.NewReader(b),
			ContentType: &ct,
		})
		if err != nil {
			return errorJSON(500, "create failed")
		}
		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Body:       string(b),
			Headers:    map[string]string{"Content-Type": "application/json"},
		}, nil
	}

	if method == http.MethodPut && strings.HasPrefix(path, "/posts/") {
		idStr := strings.TrimPrefix(path, "/posts/")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return errorJSON(400, "invalid id: must be a number")
		}
		var p Post
		if err := json.Unmarshal([]byte(req.Body), &p); err != nil {
			return errorJSON(400, "invalid body")
		}
		p.ID = id
		key := fmt.Sprintf("posts/%d.json", id)
		b, _ := json.Marshal(p)
		ct := "application/json"
		_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      &bucket,
			Key:         &key,
			Body:        bytes.NewReader(b),
			ContentType: &ct,
		})
		if err != nil {
			return errorJSON(500, "update failed")
		}
		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Body:       string(b),
			Headers:    map[string]string{"Content-Type": "application/json"},
		}, nil
	}

	if method == http.MethodDelete && strings.HasPrefix(path, "/posts/") {
		idStr := strings.TrimPrefix(path, "/posts/")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return errorJSON(400, "invalid id: must be a number")
		}
		key := fmt.Sprintf("posts/%d.json", id)
		_, err = s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &bucket, Key: &key})
		if err != nil {
			return errorJSON(500, "delete failed")
		}
		return events.APIGatewayProxyResponse{StatusCode: 204, Body: ""}, nil
	}

	return errorJSON(404, "not found")
}

func jsonOK(v interface{}) events.APIGatewayProxyResponse {
	b, _ := json.Marshal(v)
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(b),
		Headers:    map[string]string{"Content-Type": "application/json"},
	}
}

func errorJSON(code int, msg string) (events.APIGatewayProxyResponse, error) {
	b, _ := json.Marshal(map[string]string{"error": msg})
	return events.APIGatewayProxyResponse{
		StatusCode: code,
		Body:       string(b),
		Headers:    map[string]string{"Content-Type": "application/json"},
	}, nil
}

func main() {
	lambda.Start(handle)
}
```

## 必要な依存関係

この実装には以下の依存関係が必要です：

```bash
go get github.com/aws/aws-lambda-go@latest
go get github.com/aws/aws-sdk-go-v2/config@latest
go get github.com/aws/aws-sdk-go-v2/service/s3@latest
```

## API エンドポイント

### GET /posts

- **機能**: 全記事の一覧取得
- **処理**: S3の `posts/` プレフィックス配下の `.json` ファイルをすべて読み込み、JSON配列で返却
- **レスポンス**: `Post[]`

### GET /posts/{id}

- **機能**: 指定IDの記事取得
- **処理**: S3の `posts/{id}.json` を読み込んで返却
- **レスポンス**: `Post` または 404

### POST /posts

- **機能**: 新規記事作成
- **処理**: リクエストボディのJSONを `posts/{id}.json` としてS3に保存
- **リクエスト**: `Post` (id, title, content必須)
- **レスポンス**: 作成された `Post`

### PUT /posts/{id}

- **機能**: 記事更新
- **処理**: URLのIDを使用してリクエストボディを `posts/{id}.json` に上書き保存
- **リクエスト**: `Post` (title, content。idはURL優先)
- **レスポンス**: 更新された `Post`

### DELETE /posts/{id}

- **機能**: 記事削除
- **処理**: S3の `posts/{id}.json` を削除
- **レスポンス**: 204 No Content

## データ形式

### Post 構造体

```go
type Post struct {
    ID      int    `json:"id"`
    Title   string `json:"title"`
    Content string `json:"content"`
}
```

### S3 保存形式

- **キー**: `posts/{id}.json`
- **内容**: Post構造体のJSON
- **例**: `posts/1.json`
  ```json
  {
    "id": 1,
    "title": "Hello World",
    "content": "# Hello\n\nThis is my first post."
  }
  ```

## LocalStack エンドポイント設定（必要時）

Go SDK v2 のエンドポイント解決を上書きし、S3クライアントがLocalStackの `:4566` に向くようにする場合：

```go
import (
    aws "github.com/aws/aws-sdk-go-v2/aws"
)

func init() {
    bucket = os.Getenv("POSTS_BUCKET")

    endpoint := os.Getenv("AWS_ENDPOINT_URL")
    if endpoint == "" {
        // Docker Compose で localstack サービス名を解決
        endpoint = "http://localstack:4566"
    }

    resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...any) (aws.Endpoint, error) {
        return aws.Endpoint{URL: endpoint, PartitionID: "aws", SigningRegion: region}, nil
    })

    cfg, _ := config.LoadDefaultConfig(context.Background(), config.WithEndpointResolverWithOptions(resolver))
    s3Client = s3.NewFromConfig(cfg, func(o *s3.Options) { o.UsePathStyle = true })
}
```

**注意**: LocalStack の Transparent endpoint injection により明示設定が不要な場合もありますが、SDKや環境の違いで失敗する場合に有効です。

## デプロイ手順

1. **実装の置き換え**:

   ```bash
   # 上記のコードで lambda/cmd/blog/main.go を置き換え
   ```

2. **依存関係の更新**:

   ```bash
   go mod tidy
   ```

3. **ビルド & デプロイ**:
   ```bash
   mkdir -p dist/blog
   CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/blog/bootstrap lambda/cmd/blog
   ( cd dist/blog && zip -j ../blog.zip bootstrap )
   cdklocal deploy --require-approval never
   ```

## 動作確認

完全なCRUD操作の確認は[API使用方法](../guides/api-usage.md)を参照してください。

## エラーハンドリング

この実装では以下のエラーを適切に処理します：

- **400 Bad Request**: 不正なリクエスト形式
- **404 Not Found**: 存在しない記事ID
- **500 Internal Server Error**: S3操作の失敗

詳細なエラー情報は Lambda ログで確認できます：

```bash
awslocal logs tail "/aws/lambda/BlogApi" --follow
```

---

> **💡 Docker環境での開発**: 依存関係の更新が必要な場合は `docker compose exec go-dev go mod tidy` を実行してください。
