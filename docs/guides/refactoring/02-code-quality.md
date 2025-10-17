# コード品質改善

## 概要

134行の巨大なハンドラー関数を、保守性とテスト性の高い構造に改善します。

**目的**: 単一責任原則の適用、コードの可読性向上、テスト容易性の確保

**期待される効果**: 保守性の向上、バグの減少、開発効率の向上

**リスク**: APIの動作に影響を与える可能性

## 現状分析

### Before（現在の実装）

**ファイル**: `lambda/cmd/blog/main.go`（134行）

**問題点**:

- **巨大なハンドラー関数**: 134行の`handle`関数
- **責務の混在**: ルーティング、ビジネスロジック、データアクセスが混在
- **テスト困難**: 単一関数のテストが困難
- **再利用性なし**: ビジネスロジックがハンドラーに密結合
- **エラーハンドリング**: 統一されていないエラー処理

## リファクタリング手順

### 1. パッケージ構成の設計

**目的**: 責務に応じたパッケージ分離

**新しい構成**:

```
lambda/
├── cmd/
│   └── blog/
│       └── main.go          # エントリポイント（30行以下）
├── internal/
│   ├── handler/             # HTTPハンドラー
│   │   └── blog.go
│   ├── service/             # ビジネスロジック
│   │   └── blog.go
│   ├── repository/          # データアクセス
│   │   └── s3.go
│   ├── model/               # データモデル
│   │   └── post.go
│   └── response/            # レスポンス生成
│       └── response.go
└── pkg/                     # 共通ユーティリティ
    └── logger/
        └── logger.go
```

### 2. データモデルの分離

**目的**: 型定義の独立化

**ファイル**: `lambda/internal/model/post.go`

```go
package model

import "time"

// Post ブログ記事のデータモデル
type Post struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PostRequest 記事作成・更新リクエスト
type PostRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// PostListResponse 記事一覧レスポンス
type PostListResponse struct {
	Posts []Post `json:"posts"`
	Total int    `json:"total"`
}
```

### 3. レスポンス生成の統一

**目的**: HTTPレスポンス生成の統一化

**ファイル**: `lambda/internal/response/response.go`

```go
package response

import (
	"encoding/json"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
)

// Success 成功レスポンスを生成
func Success(data interface{}) events.APIGatewayProxyResponse {
	body, _ := json.Marshal(data)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(body),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}
}

// Created 作成成功レスポンスを生成
func Created(data interface{}) events.APIGatewayProxyResponse {
	body, _ := json.Marshal(data)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusCreated,
		Body:       string(body),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}
}

// NoContent 削除成功レスポンスを生成
func NoContent() events.APIGatewayProxyResponse {
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusNoContent,
		Body:       "",
	}
}

// Error エラーレスポンスを生成
func Error(code int, message string) events.APIGatewayProxyResponse {
	body, _ := json.Marshal(map[string]string{
		"error": message,
	})
	return events.APIGatewayProxyResponse{
		StatusCode: code,
		Body:       string(body),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}
}

// BadRequest バッドリクエストエラー
func BadRequest(message string) events.APIGatewayProxyResponse {
	return Error(http.StatusBadRequest, message)
}

// NotFound リソース未発見エラー
func NotFound(message string) events.APIGatewayProxyResponse {
	return Error(http.StatusNotFound, message)
}

// InternalServerError 内部サーバーエラー
func InternalServerError(message string) events.APIGatewayProxyResponse {
	return Error(http.StatusInternalServerError, message)
}
```

### 4. ロガーの導入

**目的**: 構造化ログの実装

**ファイル**: `lambda/pkg/logger/logger.go`

```go
package logger

import (
	"log/slog"
	"os"
)

var Logger *slog.Logger

func init() {
	Logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

// Info 情報ログ
func Info(msg string, args ...any) {
	Logger.Info(msg, args...)
}

// Error エラーログ
func Error(msg string, args ...any) {
	Logger.Error(msg, args...)
}

// Debug デバッグログ
func Debug(msg string, args ...any) {
	Logger.Debug(msg, args...)
}
```

### 5. リポジトリ層の実装

**目的**: データアクセス層の分離

**ファイル**: `lambda/internal/repository/s3.go`

```go
package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"lambda/internal/model"
	"lambda/pkg/logger"
)

// S3Repository S3ベースの記事リポジトリ
type S3Repository struct {
	client *s3.Client
	bucket string
}

// NewS3Repository S3リポジトリのコンストラクタ
func NewS3Repository(bucket string) (*S3Repository, error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &S3Repository{
		client: s3.NewFromConfig(cfg),
		bucket: bucket,
	}, nil
}

// ListPosts 記事一覧を取得
func (r *S3Repository) ListPosts(ctx context.Context) ([]model.Post, error) {
	logger.Info("listing posts from S3", "bucket", r.bucket)

	prefix := "posts/"
	out, err := r.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: &r.bucket,
		Prefix: &prefix,
	})
	if err != nil {
		logger.Error("failed to list objects", "error", err)
		return nil, fmt.Errorf("failed to list posts: %w", err)
	}

	posts := make([]model.Post, 0, len(out.Contents))
	for _, obj := range out.Contents {
		if !strings.HasSuffix(*obj.Key, ".json") {
			continue
		}

		post, err := r.getPostByKey(ctx, *obj.Key)
		if err != nil {
			logger.Error("failed to get post", "key", *obj.Key, "error", err)
			continue
		}
		posts = append(posts, *post)
	}

	logger.Info("successfully listed posts", "count", len(posts))
	return posts, nil
}

// GetPost 指定IDの記事を取得
func (r *S3Repository) GetPost(ctx context.Context, id int) (*model.Post, error) {
	logger.Info("getting post from S3", "id", id, "bucket", r.bucket)

	key := fmt.Sprintf("posts/%d.json", id)
	return r.getPostByKey(ctx, key)
}

// CreatePost 記事を作成
func (r *S3Repository) CreatePost(ctx context.Context, post *model.Post) error {
	logger.Info("creating post in S3", "id", post.ID, "title", post.Title)

	key := fmt.Sprintf("posts/%d.json", post.ID)
	return r.savePost(ctx, key, post)
}

// UpdatePost 記事を更新
func (r *S3Repository) UpdatePost(ctx context.Context, id int, post *model.Post) error {
	logger.Info("updating post in S3", "id", id, "title", post.Title)

	key := fmt.Sprintf("posts/%d.json", id)
	post.ID = id // URLのIDを優先
	return r.savePost(ctx, key, post)
}

// DeletePost 記事を削除
func (r *S3Repository) DeletePost(ctx context.Context, id int) error {
	logger.Info("deleting post from S3", "id", id)

	key := fmt.Sprintf("posts/%d.json", id)
	_, err := r.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &r.bucket,
		Key:    &key,
	})
	if err != nil {
		logger.Error("failed to delete post", "id", id, "error", err)
		return fmt.Errorf("failed to delete post: %w", err)
	}

	logger.Info("successfully deleted post", "id", id)
	return nil
}

// getPostByKey S3キーから記事を取得
func (r *S3Repository) getPostByKey(ctx context.Context, key string) (*model.Post, error) {
	out, err := r.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &r.bucket,
		Key:    &key,
	})
	if err != nil {
		if isNotFoundError(err) {
			return nil, fmt.Errorf("post not found")
		}
		return nil, fmt.Errorf("failed to get post: %w", err)
	}
	defer out.Body.Close()

	body, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read post body: %w", err)
	}

	var post model.Post
	if err := json.Unmarshal(body, &post); err != nil {
		return nil, fmt.Errorf("failed to unmarshal post: %w", err)
	}

	return &post, nil
}

// savePost 記事をS3に保存
func (r *S3Repository) savePost(ctx context.Context, key string, post *model.Post) error {
	body, err := json.Marshal(post)
	if err != nil {
		return fmt.Errorf("failed to marshal post: %w", err)
	}

	contentType := "application/json"
	_, err = r.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &r.bucket,
		Key:         &key,
		Body:        bytes.NewReader(body),
		ContentType: &contentType,
	})
	if err != nil {
		logger.Error("failed to save post", "key", key, "error", err)
		return fmt.Errorf("failed to save post: %w", err)
	}

	logger.Info("successfully saved post", "key", key)
	return nil
}

// isNotFoundError 404エラーの判定
func isNotFoundError(err error) bool {
	var notFound *types.NoSuchKey
	return err != nil && errors.As(err, &notFound)
}
```

### 6. サービス層の実装

**目的**: ビジネスロジックの分離

**ファイル**: `lambda/internal/service/blog.go`

```go
package service

import (
    "context"
    "fmt"
    "strconv"
    "strings"

    "lambda/internal/model"
    "lambda/pkg/logger"
)

// BlogService ブログサービスのビジネスロジック
type BlogService struct {
    repo BlogRepository
}

// BlogRepository リポジトリのインターフェース
type BlogRepository interface {
	ListPosts(ctx context.Context) ([]model.Post, error)
	GetPost(ctx context.Context, id int) (*model.Post, error)
	CreatePost(ctx context.Context, post *model.Post) error
	UpdatePost(ctx context.Context, id int, post *model.Post) error
	DeletePost(ctx context.Context, id int) error
}

// NewBlogService ブログサービスのコンストラクタ
func NewBlogService(repo BlogRepository) *BlogService {
	return &BlogService{
		repo: repo,
	}
}

// ListPosts 記事一覧を取得
func (s *BlogService) ListPosts(ctx context.Context) ([]model.Post, error) {
	logger.Info("listing posts")

	posts, err := s.repo.ListPosts(ctx)
	if err != nil {
		logger.Error("failed to list posts", "error", err)
		return nil, fmt.Errorf("failed to list posts: %w", err)
	}

	logger.Info("successfully listed posts", "count", len(posts))
	return posts, nil
}

// GetPost 指定IDの記事を取得
func (s *BlogService) GetPost(ctx context.Context, id int) (*model.Post, error) {
	logger.Info("getting post", "id", id)

	if id <= 0 {
		return nil, fmt.Errorf("invalid post ID: %d", id)
	}

	post, err := s.repo.GetPost(ctx, id)
	if err != nil {
		logger.Error("failed to get post", "id", id, "error", err)
		return nil, fmt.Errorf("failed to get post: %w", err)
	}

	logger.Info("successfully got post", "id", id, "title", post.Title)
	return post, nil
}

// CreatePost 記事を作成
func (s *BlogService) CreatePost(ctx context.Context, req *model.PostRequest) (*model.Post, error) {
	logger.Info("creating post", "title", req.Title)

	if err := s.validatePostRequest(req); err != nil {
		logger.Error("invalid post request", "error", err)
		return nil, fmt.Errorf("invalid post request: %w", err)
	}

	// IDの生成（簡易実装：既存の最大ID+1）
	id, err := s.generateNextID(ctx)
	if err != nil {
		logger.Error("failed to generate ID", "error", err)
		return nil, fmt.Errorf("failed to generate ID: %w", err)
	}

	post := &model.Post{
		ID:      id,
		Title:   req.Title,
		Content: req.Content,
	}

	if err := s.repo.CreatePost(ctx, post); err != nil {
		logger.Error("failed to create post", "error", err)
		return nil, fmt.Errorf("failed to create post: %w", err)
	}

	logger.Info("successfully created post", "id", post.ID, "title", post.Title)
	return post, nil
}

// UpdatePost 記事を更新
func (s *BlogService) UpdatePost(ctx context.Context, id int, req *model.PostRequest) (*model.Post, error) {
	logger.Info("updating post", "id", id, "title", req.Title)

	if id <= 0 {
		return nil, fmt.Errorf("invalid post ID: %d", id)
	}

	if err := s.validatePostRequest(req); err != nil {
		logger.Error("invalid post request", "error", err)
		return nil, fmt.Errorf("invalid post request: %w", err)
	}

	// 既存記事の存在確認
	existingPost, err := s.repo.GetPost(ctx, id)
	if err != nil {
		logger.Error("post not found for update", "id", id, "error", err)
		return nil, fmt.Errorf("post not found: %w", err)
	}

	// 更新データの設定
	existingPost.Title = req.Title
	existingPost.Content = req.Content

	if err := s.repo.UpdatePost(ctx, id, existingPost); err != nil {
		logger.Error("failed to update post", "id", id, "error", err)
		return nil, fmt.Errorf("failed to update post: %w", err)
	}

	logger.Info("successfully updated post", "id", id, "title", existingPost.Title)
	return existingPost, nil
}

// DeletePost 記事を削除
func (s *BlogService) DeletePost(ctx context.Context, id int) error {
	logger.Info("deleting post", "id", id)

	if id <= 0 {
		return fmt.Errorf("invalid post ID: %d", id)
	}

	// 既存記事の存在確認
	_, err := s.repo.GetPost(ctx, id)
	if err != nil {
		logger.Error("post not found for deletion", "id", id, "error", err)
		return fmt.Errorf("post not found: %w", err)
	}

	if err := s.repo.DeletePost(ctx, id); err != nil {
		logger.Error("failed to delete post", "id", id, "error", err)
		return fmt.Errorf("failed to delete post: %w", err)
	}

	logger.Info("successfully deleted post", "id", id)
	return nil
}

// validatePostRequest 記事リクエストの検証
func (s *BlogService) validatePostRequest(req *model.PostRequest) error {
	if strings.TrimSpace(req.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if strings.TrimSpace(req.Content) == "" {
		return fmt.Errorf("content is required")
	}
	return nil
}

// generateNextID 次のIDを生成（簡易実装）
func (s *BlogService) generateNextID(ctx context.Context) (int, error) {
	posts, err := s.repo.ListPosts(ctx)
	if err != nil {
		return 1, nil // エラーの場合は1から開始
	}

	maxID := 0
	for _, post := range posts {
		if post.ID > maxID {
			maxID = post.ID
		}
	}

	return maxID + 1, nil
}
```

### 7. ハンドラー層の実装

**目的**: HTTPリクエスト処理の分離

**ファイル**: `lambda/internal/handler/blog.go`

```go
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/aws/aws-lambda-go/events"

	"lambda/internal/model"
	"lambda/internal/service"
	"lambda/internal/response"
	"lambda/pkg/logger"
)

// BlogHandler ブログAPIのハンドラー
type BlogHandler struct {
	service *service.BlogService
}

// NewBlogHandler ブログハンドラーのコンストラクタ
func NewBlogHandler(service *service.BlogService) *BlogHandler {
	return &BlogHandler{
		service: service,
	}
}

// HandleRequest リクエストを処理
func (h *BlogHandler) HandleRequest(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	logger.Info("handling request",
		"method", req.HTTPMethod,
		"path", req.Path,
		"userAgent", req.Headers["User-Agent"])

	switch {
	case req.HTTPMethod == http.MethodGet && req.Path == "/posts":
		return h.handleListPosts(ctx)
	case req.HTTPMethod == http.MethodGet && strings.HasPrefix(req.Path, "/posts/"):
		return h.handleGetPost(ctx, req.Path)
	case req.HTTPMethod == http.MethodPost && req.Path == "/posts":
		return h.handleCreatePost(ctx, req.Body)
	case req.HTTPMethod == http.MethodPut && strings.HasPrefix(req.Path, "/posts/"):
		return h.handleUpdatePost(ctx, req.Path, req.Body)
	case req.HTTPMethod == http.MethodDelete && strings.HasPrefix(req.Path, "/posts/"):
		return h.handleDeletePost(ctx, req.Path)
	default:
		logger.Info("route not found", "method", req.HTTPMethod, "path", req.Path)
		return response.NotFound("route not found"), nil
	}
}

// handleListPosts 記事一覧取得
func (h *BlogHandler) handleListPosts(ctx context.Context) (events.APIGatewayProxyResponse, error) {
	posts, err := h.service.ListPosts(ctx)
	if err != nil {
		logger.Error("failed to list posts", "error", err)
		return response.InternalServerError("failed to list posts"), nil
	}

	return response.Success(posts), nil
}

// handleGetPost 記事取得
func (h *BlogHandler) handleGetPost(ctx context.Context, path string) (events.APIGatewayProxyResponse, error) {
	id, err := h.extractIDFromPath(path)
	if err != nil {
		logger.Error("invalid post ID", "path", path, "error", err)
		return response.BadRequest("invalid post ID"), nil
	}

	post, err := h.service.GetPost(ctx, id)
	if err != nil {
		logger.Error("failed to get post", "id", id, "error", err)
		if strings.Contains(err.Error(), "not found") {
			return response.NotFound("post not found"), nil
		}
		return response.InternalServerError("failed to get post"), nil
	}

	return response.Success(post), nil
}

// handleCreatePost 記事作成
func (h *BlogHandler) handleCreatePost(ctx context.Context, body string) (events.APIGatewayProxyResponse, error) {
	var req model.PostRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		logger.Error("failed to parse request body", "error", err)
		return response.BadRequest("invalid request body"), nil
	}

	post, err := h.service.CreatePost(ctx, &req)
	if err != nil {
		logger.Error("failed to create post", "error", err)
		if strings.Contains(err.Error(), "invalid") {
			return response.BadRequest(err.Error()), nil
		}
		return response.InternalServerError("failed to create post"), nil
	}

	return response.Created(post), nil
}

// handleUpdatePost 記事更新
func (h *BlogHandler) handleUpdatePost(ctx context.Context, path, body string) (events.APIGatewayProxyResponse, error) {
	id, err := h.extractIDFromPath(path)
	if err != nil {
		logger.Error("invalid post ID", "path", path, "error", err)
		return response.BadRequest("invalid post ID"), nil
	}

	var req model.PostRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		logger.Error("failed to parse request body", "error", err)
		return response.BadRequest("invalid request body"), nil
	}

	post, err := h.service.UpdatePost(ctx, id, &req)
	if err != nil {
		logger.Error("failed to update post", "id", id, "error", err)
		if strings.Contains(err.Error(), "not found") {
			return response.NotFound("post not found"), nil
		}
		if strings.Contains(err.Error(), "invalid") {
			return response.BadRequest(err.Error()), nil
		}
		return response.InternalServerError("failed to update post"), nil
	}

	return response.Success(post), nil
}

// handleDeletePost 記事削除
func (h *BlogHandler) handleDeletePost(ctx context.Context, path string) (events.APIGatewayProxyResponse, error) {
	id, err := h.extractIDFromPath(path)
	if err != nil {
		logger.Error("invalid post ID", "path", path, "error", err)
		return response.BadRequest("invalid post ID"), nil
	}

	err = h.service.DeletePost(ctx, id)
	if err != nil {
		logger.Error("failed to delete post", "id", id, "error", err)
		if strings.Contains(err.Error(), "not found") {
			return response.NotFound("post not found"), nil
		}
		return response.InternalServerError("failed to delete post"), nil
	}

	return response.NoContent(), nil
}

// extractIDFromPath パスからIDを抽出
func (h *BlogHandler) extractIDFromPath(path string) (int, error) {
	idStr := strings.TrimPrefix(path, "/posts/")
	return strconv.Atoi(idStr)
}
```

### 8. メイン関数の簡素化

**目的**: エントリポイントの簡素化

**ファイル**: `lambda/cmd/blog/main.go`

```go
package main

import (
	"context"
	"os"

	"github.com/aws/aws-lambda-go/lambda"

	"lambda/internal/handler"
	"lambda/internal/repository"
	"lambda/internal/service"
	"lambda/pkg/logger"
)

func main() {
	logger.Info("starting blog API")

	// 環境変数の取得
	bucket := os.Getenv("POSTS_BUCKET")
	if bucket == "" {
		logger.Error("POSTS_BUCKET environment variable is required")
		os.Exit(1)
	}

	// リポジトリの初期化
	repo, err := repository.NewS3Repository(bucket)
	if err != nil {
		logger.Error("failed to initialize repository", "error", err)
		os.Exit(1)
	}

	// サービスの初期化
	blogService := service.NewBlogService(repo)

	// ハンドラーの初期化
	blogHandler := handler.NewBlogHandler(blogService)

	// Lambda関数の開始
	lambda.Start(blogHandler.HandleRequest)
}
```

### 9. 依存関係の更新

**目的**: 新しいパッケージ構成に対応

```bash
# go.modの更新
go mod tidy

# 依存関係の確認
go mod graph
```

## 動作確認

### ビルドとデプロイ

```bash
# 新しい構造でビルド
mkdir -p dist/blog
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/blog/bootstrap ./lambda/cmd/blog
cd dist/blog && zip -j ../blog.zip bootstrap

# デプロイ
cdklocal deploy --require-approval never
```

### API動作確認

```bash
# APIエンドポイントの確認
REGION=${AWS_DEFAULT_REGION:-ap-northeast-1}
REST_API_ID=$(awslocal --region "$REGION" apigateway get-rest-apis | jq -r '.items[0].id')
BASE="http://localhost:4566/restapis/${REST_API_ID}/prod/_user_request_"

# 1. 記事一覧取得
echo "Testing GET /posts"
curl -s "${BASE}/posts" | jq .

# 2. 記事作成
echo "Testing POST /posts"
curl -s -X POST \
  -H "Content-Type: application/json" \
  -d '{"title":"Refactored Post","content":"This is a refactored post."}' \
  "${BASE}/posts" | jq .

# 3. 記事取得
echo "Testing GET /posts/1"
curl -s "${BASE}/posts/1" | jq .
```

### 期待結果

- **コード行数**: 134行 → 30行以下（main.go）
- **パッケージ数**: 1個 → 6個
- **関数の責務**: 単一責任原則に準拠
- **テスト容易性**: 各層が独立してテスト可能
- **ログ**: 構造化ログでデバッグ情報が充実

## トラブルシューティング

### ビルドエラー

**症状**: `go build`でパッケージが見つからない

**原因**: 新しいパッケージ構成でimportパスが変更

**解決策**:

```bash
# パッケージパスの確認
go list ./lambda/...

# 依存関係の更新
go mod tidy
```

### デプロイエラー

**症状**: Lambda関数が起動しない

**原因**: 新しいパッケージ構成でランタイムエラー

**解決策**:

```bash
# ローカルでテスト
go run ./lambda/cmd/blog

# ログの確認
awslocal logs tail "/aws/lambda/BlogApi" --follow
```

### API応答エラー

**症状**: 500エラーが発生

**原因**: サービス層でのエラーハンドリング

**解決策**:

```bash
# 詳細ログの確認
awslocal logs tail "/aws/lambda/BlogApi" --follow

# 環境変数の確認
awslocal lambda get-function --function-name BlogApi | jq '.Configuration.Environment'
```

## 次のステップ

コード品質改善が完了したら、[パフォーマンス最適化](../refactoring/03-performance.md)に進んでください。

**完了確認**:

- [ ] パッケージ構成が整理されている
- [ ] 各層の責務が分離されている
- [ ] エラーハンドリングが統一されている
- [ ] 構造化ログが実装されている
- [ ] APIが正常に動作している

---

> **💡 ヒント**: コード品質改善は段階的に進めることが重要です。一度にすべてを変更せず、各層を順番に実装して動作確認を行ってください。
