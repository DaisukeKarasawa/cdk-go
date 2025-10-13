# テスト追加

## 概要

リファクタリング後のアプリケーションに包括的なテストスイートを追加し、品質と信頼性を確保します。

**目的**: テストカバレッジの向上、回帰テストの実装、CI/CDパイプラインの準備

**期待される効果**: バグの早期発見、リファクタリングの安全性向上、継続的な品質保証

**リスク**: テストの保守コスト、テストデータの管理

## 現状分析

### Before（テスト追加前の状態）

**テスト状況**:

- **ユニットテスト**: なし
- **統合テスト**: なし
- **E2Eテスト**: なし
- **テストカバレッジ**: 0%
- **CI/CD**: なし

**課題**:

- リファクタリング時の回帰リスク
- バグの早期発見が困難
- 手動テストに依存
- 品質の定量評価ができない

## リファクタリング手順

### 1. テスト環境のセットアップ

**目的**: テスト実行環境の整備

**ファイル**: `lambda/internal/test/testutils.go`

```go
package test

import (
	"context"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"lambda/internal/model"
)

// MockS3Client S3クライアントのモック
type MockS3Client struct {
	objects map[string][]byte
	errors  map[string]error
}

// NewMockS3Client モックS3クライアントのコンストラクタ
func NewMockS3Client() *MockS3Client {
	return &MockS3Client{
		objects: make(map[string][]byte),
		errors:  make(map[string]error),
	}
}

// SetObject オブジェクトを設定
func (m *MockS3Client) SetObject(key string, data []byte) {
	m.objects[key] = data
}

// SetError エラーを設定
func (m *MockS3Client) SetError(key string, err error) {
	m.errors[key] = err
}

// GetObject オブジェクトを取得
func (m *MockS3Client) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	key := *params.Key

	if err, exists := m.errors[key]; exists {
		return nil, err
	}

	if data, exists := m.objects[key]; exists {
		return &s3.GetObjectOutput{
			Body: &mockReadCloser{data: data},
		}, nil
	}

	return nil, &types.NoSuchKey{}
}

// PutObject オブジェクトを保存
func (m *MockS3Client) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	key := *params.Key

	if err, exists := m.errors[key]; exists {
		return nil, err
	}

	// ボディを読み取り
	data := make([]byte, 1024)
	n, _ := params.Body.Read(data)
	m.objects[key] = data[:n]

	return &s3.PutObjectOutput{}, nil
}

// ListObjectsV2 オブジェクト一覧を取得
func (m *MockS3Client) ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	var contents []types.Object

	for key := range m.objects {
		if *params.Prefix == "" || len(key) >= len(*params.Prefix) && key[:len(*params.Prefix)] == *params.Prefix {
			contents = append(contents, types.Object{
				Key: &key,
			})
		}
	}

	return &s3.ListObjectsV2Output{
		Contents: contents,
	}, nil
}

// DeleteObject オブジェクトを削除
func (m *MockS3Client) DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	key := *params.Key

	if err, exists := m.errors[key]; exists {
		return nil, err
	}

	delete(m.objects, key)
	return &s3.DeleteObjectOutput{}, nil
}

// mockReadCloser モック用のReadCloser
type mockReadCloser struct {
	data []byte
	pos  int
}

func (m *mockReadCloser) Read(p []byte) (n int, err error) {
	if m.pos >= len(m.data) {
		return 0, io.EOF
	}

	n = copy(p, m.data[m.pos:])
	m.pos += n
	return n, nil
}

func (m *mockReadCloser) Close() error {
	return nil
}

// CreateTestPost テスト用の記事を作成
func CreateTestPost(id int, title, content string) *model.Post {
	return &model.Post{
		ID:      id,
		Title:   title,
		Content: content,
	}
}

// CreateTestRequest テスト用のリクエストを作成
func CreateTestRequest(title, content string) *model.PostRequest {
	return &model.PostRequest{
		Title:   title,
		Content: content,
	}
}

// CreateTestAPIGatewayRequest テスト用のAPI Gatewayリクエストを作成
func CreateTestAPIGatewayRequest(method, path, body string) events.APIGatewayProxyRequest {
	return events.APIGatewayProxyRequest{
		HTTPMethod: method,
		Path:       path,
		Body:       body,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}
}
```

### 2. ユニットテストの実装

**目的**: 各層の個別テスト

**ファイル**: `lambda/internal/service/blog_test.go`

```go
package service

import (
	"context"
	"errors"
	"testing"

	"lambda/internal/model"
	"lambda/internal/test"
)

// MockRepository リポジトリのモック
type MockRepository struct {
	posts map[int]*model.Post
	nextID int
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		posts:  make(map[int]*model.Post),
		nextID: 1,
	}
}

func (m *MockRepository) ListPosts(ctx context.Context) ([]model.Post, error) {
	var posts []model.Post
	for _, post := range m.posts {
		posts = append(posts, *post)
	}
	return posts, nil
}

func (m *MockRepository) GetPost(ctx context.Context, id int) (*model.Post, error) {
	if post, exists := m.posts[id]; exists {
		return post, nil
	}
	return nil, errors.New("post not found")
}

func (m *MockRepository) CreatePost(ctx context.Context, post *model.Post) error {
	post.ID = m.nextID
	m.posts[m.nextID] = post
	m.nextID++
	return nil
}

func (m *MockRepository) UpdatePost(ctx context.Context, id int, post *model.Post) error {
	if _, exists := m.posts[id]; !exists {
		return errors.New("post not found")
	}
	post.ID = id
	m.posts[id] = post
	return nil
}

func (m *MockRepository) DeletePost(ctx context.Context, id int) error {
	if _, exists := m.posts[id]; !exists {
		return errors.New("post not found")
	}
	delete(m.posts, id)
	return nil
}

func TestBlogService_ListPosts(t *testing.T) {
	// セットアップ
	repo := NewMockRepository()
	service := NewBlogService(repo)
	ctx := context.Background()

	// テストデータの準備
	post1 := test.CreateTestPost(1, "Test Post 1", "Content 1")
	post2 := test.CreateTestPost(2, "Test Post 2", "Content 2")
	repo.CreatePost(ctx, post1)
	repo.CreatePost(ctx, post2)

	// テスト実行
	posts, err := service.ListPosts(ctx)

	// 検証
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("Expected 2 posts, got %d", len(posts))
	}
}

func TestBlogService_GetPost(t *testing.T) {
	// セットアップ
	repo := NewMockRepository()
	service := NewBlogService(repo)
	ctx := context.Background()

	// テストデータの準備
	post := test.CreateTestPost(1, "Test Post", "Test Content")
	repo.CreatePost(ctx, post)

	// テスト実行
	result, err := service.GetPost(ctx, 1)

	// 検証
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result.Title != "Test Post" {
		t.Fatalf("Expected title 'Test Post', got '%s'", result.Title)
	}
}

func TestBlogService_GetPost_NotFound(t *testing.T) {
	// セットアップ
	repo := NewMockRepository()
	service := NewBlogService(repo)
	ctx := context.Background()

	// テスト実行
	_, err := service.GetPost(ctx, 999)

	// 検証
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !errors.Is(err, errors.New("post not found")) {
		t.Fatalf("Expected 'post not found' error, got %v", err)
	}
}

func TestBlogService_CreatePost(t *testing.T) {
	// セットアップ
	repo := NewMockRepository()
	service := NewBlogService(repo)
	ctx := context.Background()

	// テストデータの準備
	req := test.CreateTestRequest("New Post", "New Content")

	// テスト実行
	post, err := service.CreatePost(ctx, req)

	// 検証
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if post.Title != "New Post" {
		t.Fatalf("Expected title 'New Post', got '%s'", post.Title)
	}
	if post.ID == 0 {
		t.Fatal("Expected non-zero ID")
	}
}

func TestBlogService_CreatePost_InvalidRequest(t *testing.T) {
	// セットアップ
	repo := NewMockRepository()
	service := NewBlogService(repo)
	ctx := context.Background()

	// テストデータの準備（無効なリクエスト）
	req := test.CreateTestRequest("", "Content") // タイトルが空

	// テスト実行
	_, err := service.CreatePost(ctx, req)

	// 検証
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

func TestBlogService_UpdatePost(t *testing.T) {
	// セットアップ
	repo := NewMockRepository()
	service := NewBlogService(repo)
	ctx := context.Background()

	// テストデータの準備
	originalPost := test.CreateTestPost(1, "Original Title", "Original Content")
	repo.CreatePost(ctx, originalPost)

	req := test.CreateTestRequest("Updated Title", "Updated Content")

	// テスト実行
	updatedPost, err := service.UpdatePost(ctx, 1, req)

	// 検証
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if updatedPost.Title != "Updated Title" {
		t.Fatalf("Expected title 'Updated Title', got '%s'", updatedPost.Title)
	}
	if updatedPost.ID != 1 {
		t.Fatalf("Expected ID 1, got %d", updatedPost.ID)
	}
}

func TestBlogService_DeletePost(t *testing.T) {
	// セットアップ
	repo := NewMockRepository()
	service := NewBlogService(repo)
	ctx := context.Background()

	// テストデータの準備
	post := test.CreateTestPost(1, "Test Post", "Test Content")
	repo.CreatePost(ctx, post)

	// テスト実行
	err := service.DeletePost(ctx, 1)

	// 検証
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// 削除確認
	_, err = service.GetPost(ctx, 1)
	if err == nil {
		t.Fatal("Expected error after deletion, got nil")
	}
}
```

**ファイル**: `lambda/internal/handler/blog_test.go`

```go
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"

	"lambda/internal/model"
	"lambda/internal/service"
	"lambda/internal/test"
)

func TestBlogHandler_HandleRequest_ListPosts(t *testing.T) {
	// セットアップ
	repo := service.NewMockRepository()
	blogService := service.NewBlogService(repo)
	handler := NewBlogHandler(blogService)
	ctx := context.Background()

	// テストデータの準備
	post := test.CreateTestPost(1, "Test Post", "Test Content")
	repo.CreatePost(ctx, post)

	// テスト実行
	req := test.CreateTestAPIGatewayRequest(http.MethodGet, "/posts", "")
	resp, err := handler.HandleRequest(ctx, req)

	// 検証
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	var posts []model.Post
	if err := json.Unmarshal([]byte(resp.Body), &posts); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("Expected 1 post, got %d", len(posts))
	}
}

func TestBlogHandler_HandleRequest_GetPost(t *testing.T) {
	// セットアップ
	repo := service.NewMockRepository()
	blogService := service.NewBlogService(repo)
	handler := NewBlogHandler(blogService)
	ctx := context.Background()

	// テストデータの準備
	post := test.CreateTestPost(1, "Test Post", "Test Content")
	repo.CreatePost(ctx, post)

	// テスト実行
	req := test.CreateTestAPIGatewayRequest(http.MethodGet, "/posts/1", "")
	resp, err := handler.HandleRequest(ctx, req)

	// 検証
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	var result model.Post
	if err := json.Unmarshal([]byte(resp.Body), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if result.Title != "Test Post" {
		t.Fatalf("Expected title 'Test Post', got '%s'", result.Title)
	}
}

func TestBlogHandler_HandleRequest_CreatePost(t *testing.T) {
	// セットアップ
	repo := service.NewMockRepository()
	blogService := service.NewBlogService(repo)
	handler := NewBlogHandler(blogService)
	ctx := context.Background()

	// テストデータの準備
	reqBody := `{"title":"New Post","content":"New Content"}`
	req := test.CreateTestAPIGatewayRequest(http.MethodPost, "/posts", reqBody)

	// テスト実行
	resp, err := handler.HandleRequest(ctx, req)

	// 検証
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d", resp.StatusCode)
	}

	var result model.Post
	if err := json.Unmarshal([]byte(resp.Body), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if result.Title != "New Post" {
		t.Fatalf("Expected title 'New Post', got '%s'", result.Title)
	}
}

func TestBlogHandler_HandleRequest_NotFound(t *testing.T) {
	// セットアップ
	repo := service.NewMockRepository()
	blogService := service.NewBlogService(repo)
	handler := NewBlogHandler(blogService)
	ctx := context.Background()

	// テスト実行
	req := test.CreateTestAPIGatewayRequest(http.MethodGet, "/posts/999", "")
	resp, err := handler.HandleRequest(ctx, req)

	// 検証
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Expected status 404, got %d", resp.StatusCode)
	}
}
```

### 3. 統合テストの実装

**目的**: 層間の連携テスト

**ファイル**: `lambda/internal/integration/blog_integration_test.go`

```go
package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"

	"lambda/internal/handler"
	"lambda/internal/repository"
	"lambda/internal/service"
	"lambda/internal/test"
)

func TestBlogIntegration_FullWorkflow(t *testing.T) {
	// セットアップ
	mockS3 := test.NewMockS3Client()
	repo, err := repository.NewS3RepositoryWithClient("test-bucket", mockS3)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	blogService := service.NewBlogService(repo)
	handler := handler.NewBlogHandler(blogService)
	ctx := context.Background()

	// 1. 記事作成
	createReq := test.CreateTestAPIGatewayRequest(
		http.MethodPost,
		"/posts",
		`{"title":"Integration Test","content":"Integration test content"}`,
	)

	createResp, err := handler.HandleRequest(ctx, createReq)
	if err != nil {
		t.Fatalf("Failed to create post: %v", err)
	}
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d", createResp.StatusCode)
	}

	// 2. 記事一覧取得
	listReq := test.CreateTestAPIGatewayRequest(http.MethodGet, "/posts", "")
	listResp, err := handler.HandleRequest(ctx, listReq)
	if err != nil {
		t.Fatalf("Failed to list posts: %v", err)
	}
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", listResp.StatusCode)
	}

	// 3. 記事取得
	getReq := test.CreateTestAPIGatewayRequest(http.MethodGet, "/posts/1", "")
	getResp, err := handler.HandleRequest(ctx, getReq)
	if err != nil {
		t.Fatalf("Failed to get post: %v", err)
	}
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", getResp.StatusCode)
	}

	// 4. 記事更新
	updateReq := test.CreateTestAPIGatewayRequest(
		http.MethodPut,
		"/posts/1",
		`{"title":"Updated Title","content":"Updated content"}`,
	)

	updateResp, err := handler.HandleRequest(ctx, updateReq)
	if err != nil {
		t.Fatalf("Failed to update post: %v", err)
	}
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", updateResp.StatusCode)
	}

	// 5. 記事削除
	deleteReq := test.CreateTestAPIGatewayRequest(http.MethodDelete, "/posts/1", "")
	deleteResp, err := handler.HandleRequest(ctx, deleteReq)
	if err != nil {
		t.Fatalf("Failed to delete post: %v", err)
	}
	if deleteResp.StatusCode != http.StatusNoContent {
		t.Fatalf("Expected status 204, got %d", deleteResp.StatusCode)
	}

	// 6. 削除確認
	getAfterDeleteReq := test.CreateTestAPIGatewayRequest(http.MethodGet, "/posts/1", "")
	getAfterDeleteResp, err := handler.HandleRequest(ctx, getAfterDeleteReq)
	if err != nil {
		t.Fatalf("Failed to get post after deletion: %v", err)
	}
	if getAfterDeleteResp.StatusCode != http.StatusNotFound {
		t.Fatalf("Expected status 404, got %d", getAfterDeleteResp.StatusCode)
	}
}
```

### 4. E2Eテストの実装

**目的**: LocalStackを使ったエンドツーエンドテスト

**ファイル**: `lambda/e2e/blog_e2e_test.go`

```go
package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"lambda/internal/handler"
	"lambda/internal/repository"
	"lambda/internal/service"
)

func TestBlogE2E_FullAPI(t *testing.T) {
	// LocalStack環境の確認
	if os.Getenv("POSTS_BUCKET") == "" {
		t.Skip("Skipping E2E test: POSTS_BUCKET not set")
	}

	// セットアップ
	repo, err := repository.NewS3Repository(os.Getenv("POSTS_BUCKET"))
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	blogService := service.NewBlogService(repo)
	handler := handler.NewBlogHandler(blogService)
	ctx := context.Background()

	// Lambda関数の起動
	go lambda.Start(handler.HandleRequest)
	time.Sleep(2 * time.Second) // 起動待機

	// テスト実行
	t.Run("CreatePost", func(t *testing.T) {
		req := events.APIGatewayProxyRequest{
			HTTPMethod: http.MethodPost,
			Path:       "/posts",
			Body:       `{"title":"E2E Test","content":"E2E test content"}`,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		}

		resp, err := handler.HandleRequest(ctx, req)
		if err != nil {
			t.Fatalf("Failed to create post: %v", err)
		}
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("Expected status 201, got %d", resp.StatusCode)
		}
	})

	t.Run("ListPosts", func(t *testing.T) {
		req := events.APIGatewayProxyRequest{
			HTTPMethod: http.MethodGet,
			Path:       "/posts",
		}

		resp, err := handler.HandleRequest(ctx, req)
		if err != nil {
			t.Fatalf("Failed to list posts: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("GetPost", func(t *testing.T) {
		req := events.APIGatewayProxyRequest{
			HTTPMethod: http.MethodGet,
			Path:       "/posts/1",
		}

		resp, err := handler.HandleRequest(ctx, req)
		if err != nil {
			t.Fatalf("Failed to get post: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("UpdatePost", func(t *testing.T) {
		req := events.APIGatewayProxyRequest{
			HTTPMethod: http.MethodPut,
			Path:       "/posts/1",
			Body:       `{"title":"Updated E2E Test","content":"Updated E2E content"}`,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		}

		resp, err := handler.HandleRequest(ctx, req)
		if err != nil {
			t.Fatalf("Failed to update post: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("DeletePost", func(t *testing.T) {
		req := events.APIGatewayProxyRequest{
			HTTPMethod: http.MethodDelete,
			Path:       "/posts/1",
		}

		resp, err := handler.HandleRequest(ctx, req)
		if err != nil {
			t.Fatalf("Failed to delete post: %v", err)
		}
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("Expected status 204, got %d", resp.StatusCode)
		}
	})
}
```

### 5. テスト実行スクリプト

**目的**: テストの自動実行

**ファイル**: `scripts/test.sh`

```bash
#!/bin/bash

set -e

echo "🧪 Running Blog API Tests"

# 環境変数の設定
export POSTS_BUCKET="test-bucket"
export AWS_DEFAULT_REGION="ap-northeast-1"
export AWS_ACCESS_KEY_ID="dummy"
export AWS_SECRET_ACCESS_KEY="dummy"

# テストカバレッジの設定
export COVERAGE_DIR="coverage"

# カバレッジディレクトリの作成
mkdir -p $COVERAGE_DIR

echo "📊 Running unit tests..."
go test -v -coverprofile=$COVERAGE_DIR/unit.out ./lambda/internal/service/...
go test -v -coverprofile=$COVERAGE_DIR/handler.out ./lambda/internal/handler/...

echo "🔗 Running integration tests..."
go test -v -coverprofile=$COVERAGE_DIR/integration.out ./lambda/internal/integration/...

echo "🌐 Running E2E tests..."
go test -v -coverprofile=$COVERAGE_DIR/e2e.out ./lambda/e2e/...

echo "📈 Generating coverage report..."
go tool cover -html=$COVERAGE_DIR/unit.out -o $COVERAGE_DIR/unit.html
go tool cover -html=$COVERAGE_DIR/handler.out -o $COVERAGE_DIR/handler.html
go tool cover -html=$COVERAGE_DIR/integration.out -o $COVERAGE_DIR/integration.html
go tool cover -html=$COVERAGE_DIR/e2e.out -o $COVERAGE_DIR/e2e.html

# 全体のカバレッジを結合
echo "mode: set" > $COVERAGE_DIR/all.out
cat $COVERAGE_DIR/unit.out | grep -v "mode:" >> $COVERAGE_DIR/all.out
cat $COVERAGE_DIR/handler.out | grep -v "mode:" >> $COVERAGE_DIR/all.out
cat $COVERAGE_DIR/integration.out | grep -v "mode:" >> $COVERAGE_DIR/all.out
cat $COVERAGE_DIR/e2e.out | grep -v "mode:" >> $COVERAGE_DIR/all.out

go tool cover -html=$COVERAGE_DIR/all.out -o $COVERAGE_DIR/all.html

echo "✅ All tests completed successfully!"
echo "📊 Coverage report generated in $COVERAGE_DIR/"
```

### 6. Makefileの更新

**目的**: テスト実行の簡素化

**ファイル**: `Makefile`（更新）

```makefile
# テスト実行
test:
	@echo "🧪 Running all tests..."
	go test -v ./lambda/internal/...

test-unit:
	@echo "🧪 Running unit tests..."
	go test -v ./lambda/internal/service/... ./lambda/internal/handler/...

test-integration:
	@echo "🔗 Running integration tests..."
	go test -v ./lambda/internal/integration/...

test-e2e:
	@echo "🌐 Running E2E tests..."
	go test -v ./lambda/e2e/...

test-coverage:
	@echo "📊 Running tests with coverage..."
	./scripts/test.sh

test-docker:
	@echo "🧪 Running tests in Docker..."
	docker compose exec go-dev go test -v ./lambda/internal/...

# ベンチマークテスト
benchmark:
	@echo "⚡ Running benchmark tests..."
	go test -bench=. ./lambda/internal/...

# テストデータのクリーンアップ
test-clean:
	@echo "🧹 Cleaning up test data..."
	awslocal s3 rm s3://test-bucket/posts/ --recursive || true
```

## 動作確認

### テスト実行

```bash
# ユニットテスト
make test-unit

# 統合テスト
make test-integration

# E2Eテスト
make test-e2e

# カバレッジ付きテスト
make test-coverage
```

### 期待結果

- **ユニットテスト**: 20個以上のテストケース
- **統合テスト**: 5個以上のテストケース
- **E2Eテスト**: 3個以上のテストケース
- **テストカバレッジ**: 80%以上
- **テスト実行時間**: 30秒以内

## トラブルシューティング

### テストが失敗する

**症状**: ユニットテストでエラー

**原因**: モックの設定不備

**解決策**:

```go
// モックの初期化を確認
mockRepo := service.NewMockRepository()
mockRepo.SetError("posts/1.json", errors.New("not found"))
```

### E2Eテストが失敗する

**症状**: LocalStack接続エラー

**原因**: 環境変数の設定不備

**解決策**:

```bash
# 環境変数の確認
echo $POSTS_BUCKET
echo $AWS_DEFAULT_REGION

# LocalStackの起動確認
docker compose ps
```

### カバレッジが低い

**症状**: カバレッジが80%未満

**原因**: テストケースの不足

**解決策**:

```bash
# カバレッジレポートの確認
go tool cover -html=coverage/all.out

# 未カバーの部分を特定してテストを追加
```

## 次のステップ

テスト追加が完了したら、[インフラ改善](../refactoring/05-infrastructure.md)に進んでください。

**完了確認**:

- [ ] ユニットテストが実装されている
- [ ] 統合テストが実装されている
- [ ] E2Eテストが実装されている
- [ ] テストカバレッジが80%以上
- [ ] テスト実行スクリプトが動作している

---

> **💡 ヒント**: テストは段階的に追加していくことが重要です。まずユニットテストから始めて、徐々に統合テスト、E2Eテストを追加してください。
