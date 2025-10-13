# パフォーマンス最適化

## 概要

コード品質改善後のアプリケーションを、より高速で効率的なシステムに最適化します。

**目的**: Cold Start時間の短縮、API応答時間の改善、メモリ使用量の最適化

**期待される効果**: ユーザー体験の向上、コスト削減、スケーラビリティの向上

**リスク**: 最適化による複雑性の増加、デバッグの困難化

## 現状分析

### Before（最適化前の状態）

**パフォーマンス課題**:

- **Cold Start**: Lambda関数の初回起動に時間がかかる
- **S3アクセス**: 毎回S3への接続とデータ取得
- **メモリ使用量**: 不要なメモリ消費
- **並行処理**: 複数リクエストの効率的な処理
- **キャッシング**: データの再利用ができない

**測定例**:

```
Cold Start時間: 2.5秒
API応答時間: 800ms
メモリ使用量: 128MB
```

## リファクタリング手順

### 1. Lambda設定の最適化

**目的**: Lambda関数の実行環境を最適化

**CDKスタックの更新**: `cdk-go.go`

```go
// Lambda関数の設定を最適化
fn := awslambda.NewFunction(stack, jsii.String("BlogApi"), &awslambda.FunctionProps{
	Runtime:      awslambda.Runtime_PROVIDED_AL2(),
	Handler:      jsii.String("bootstrap"),
	Code:         awslambda.Code_FromAsset(jsii.String("dist/blog.zip"), nil),

	// パフォーマンス最適化設定
	MemorySize:   jsii.Number(256),        // メモリを256MBに増加
	Timeout:      awscdk.Duration_Seconds(jsii.Number(30)), // タイムアウトを30秒に設定

	// 環境変数
	Environment: &map[string]*string{
		"POSTS_BUCKET": bucket.BucketName(),
		"LOG_LEVEL":    jsii.String("INFO"),
		"ENABLE_CACHE": jsii.String("true"),
	},

	// 並行実行設定
	ReservedConcurrentExecutions: jsii.Number(10), // 同時実行数を制限

	// デッドレターキュー（エラー処理）
	DeadLetterQueue: awssqs.NewQueue(stack, jsii.String("BlogApiDLQ"), &awssqs.QueueProps{
		QueueName: jsii.String("blog-api-dlq"),
	}),
})

// デッドレターキューの権限付与
fn.AddDeadLetterQueue(fn.DeadLetterQueue())
```

### 2. S3クライアントの最適化

**目的**: S3アクセスの効率化

**ファイル**: `lambda/internal/repository/s3.go`（更新）

```go
package repository

import (
	"context"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Repository S3ベースの記事リポジトリ（最適化版）
type S3Repository struct {
	client     *s3.Client
	bucket     string
	clientOnce sync.Once
}

// NewS3Repository S3リポジトリのコンストラクタ（最適化版）
func NewS3Repository(bucket string) (*S3Repository, error) {
	repo := &S3Repository{
		bucket: bucket,
	}

	// クライアントの遅延初期化
	repo.clientOnce.Do(func() {
		cfg, err := config.LoadDefaultConfig(context.Background(),
			config.WithRetryer(func() aws.Retryer {
				return aws.NopRetryer{} // リトライを無効化（LocalStack用）
			}),
		)
		if err != nil {
			logger.Error("failed to load AWS config", "error", err)
			return
		}

		repo.client = s3.NewFromConfig(cfg, func(o *s3.Options) {
			// 接続プールの最適化
			o.UsePathStyle = true // LocalStack用
		})
	})

	return repo, nil
}

// ListPosts 記事一覧を取得（最適化版）
func (r *S3Repository) ListPosts(ctx context.Context) ([]model.Post, error) {
	logger.Info("listing posts from S3", "bucket", r.bucket)

	// タイムアウト付きコンテキスト
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	prefix := "posts/"
	out, err := r.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  &r.bucket,
		Prefix:  &prefix,
		MaxKeys: aws.Int32(1000), // 最大取得数を制限
	})
	if err != nil {
		logger.Error("failed to list objects", "error", err)
		return nil, fmt.Errorf("failed to list posts: %w", err)
	}

	// 並行処理で記事を取得
	posts := make([]model.Post, 0, len(out.Contents))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, obj := range out.Contents {
		if !strings.HasSuffix(*obj.Key, ".json") {
			continue
		}

		wg.Add(1)
		go func(key string) {
			defer wg.Done()

			post, err := r.getPostByKey(ctx, key)
			if err != nil {
				logger.Error("failed to get post", "key", key, "error", err)
				return
			}

			mu.Lock()
			posts = append(posts, *post)
			mu.Unlock()
		}(*obj.Key)
	}

	wg.Wait()

	logger.Info("successfully listed posts", "count", len(posts))
	return posts, nil
}
```

### 3. メモリキャッシュの実装

**目的**: 頻繁にアクセスされるデータのキャッシュ

**ファイル**: `lambda/internal/cache/memory.go`

```go
package cache

import (
	"sync"
	"time"

	"lambda/internal/model"
	"lambda/pkg/logger"
)

// CacheItem キャッシュアイテム
type CacheItem struct {
	Data      interface{}
	ExpiresAt time.Time
}

// MemoryCache メモリキャッシュ
type MemoryCache struct {
	items map[string]CacheItem
	mutex sync.RWMutex
	ttl   time.Duration
}

// NewMemoryCache メモリキャッシュのコンストラクタ
func NewMemoryCache(ttl time.Duration) *MemoryCache {
	cache := &MemoryCache{
		items: make(map[string]CacheItem),
		ttl:   ttl,
	}

	// 定期的なクリーンアップ
	go cache.cleanup()

	return cache
}

// Get キャッシュからデータを取得
func (c *MemoryCache) Get(key string) (interface{}, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	item, exists := c.items[key]
	if !exists || time.Now().After(item.ExpiresAt) {
		return nil, false
	}

	return item.Data, true
}

// Set キャッシュにデータを設定
func (c *MemoryCache) Set(key string, data interface{}) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.items[key] = CacheItem{
		Data:      data,
		ExpiresAt: time.Now().Add(c.ttl),
	}
}

// Delete キャッシュからデータを削除
func (c *MemoryCache) Delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.items, key)
}

// Clear キャッシュをクリア
func (c *MemoryCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.items = make(map[string]CacheItem)
}

// cleanup 期限切れアイテムのクリーンアップ
func (c *MemoryCache) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mutex.Lock()
		now := time.Now()
		for key, item := range c.items {
			if now.After(item.ExpiresAt) {
				delete(c.items, key)
			}
		}
		c.mutex.Unlock()

		logger.Debug("cache cleanup completed", "items", len(c.items))
	}
}

// Size キャッシュサイズを取得
func (c *MemoryCache) Size() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return len(c.items)
}
```

### 4. キャッシュ対応サービスの実装

**目的**: サービス層にキャッシュ機能を統合

**ファイル**: `lambda/internal/service/blog.go`（更新）

```go
package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"lambda/internal/cache"
	"lambda/internal/model"
	"lambda/pkg/logger"
)

// BlogService ブログサービスのビジネスロジック（キャッシュ対応）
type BlogService struct {
	repo  BlogRepository
	cache *cache.MemoryCache
}

// NewBlogService ブログサービスのコンストラクタ（キャッシュ対応）
func NewBlogService(repo BlogRepository) *BlogService {
	return &BlogService{
		repo:  repo,
		cache: cache.NewMemoryCache(5 * time.Minute), // 5分のTTL
	}
}

// ListPosts 記事一覧を取得（キャッシュ対応）
func (s *BlogService) ListPosts(ctx context.Context) ([]model.Post, error) {
	logger.Info("listing posts")

	// キャッシュから取得を試行
	cacheKey := "posts:list"
	if cached, found := s.cache.Get(cacheKey); found {
		logger.Info("posts retrieved from cache")
		return cached.([]model.Post), nil
	}

	// キャッシュにない場合はリポジトリから取得
	posts, err := s.repo.ListPosts(ctx)
	if err != nil {
		logger.Error("failed to list posts", "error", err)
		return nil, fmt.Errorf("failed to list posts: %w", err)
	}

	// キャッシュに保存
	s.cache.Set(cacheKey, posts)

	logger.Info("successfully listed posts", "count", len(posts))
	return posts, nil
}

// GetPost 指定IDの記事を取得（キャッシュ対応）
func (s *BlogService) GetPost(ctx context.Context, id int) (*model.Post, error) {
	logger.Info("getting post", "id", id)

	if id <= 0 {
		return nil, fmt.Errorf("invalid post ID: %d", id)
	}

	// キャッシュから取得を試行
	cacheKey := fmt.Sprintf("post:%d", id)
	if cached, found := s.cache.Get(cacheKey); found {
		logger.Info("post retrieved from cache", "id", id)
		return cached.(*model.Post), nil
	}

	// キャッシュにない場合はリポジトリから取得
	post, err := s.repo.GetPost(ctx, id)
	if err != nil {
		logger.Error("failed to get post", "id", id, "error", err)
		return nil, fmt.Errorf("failed to get post: %w", err)
	}

	// キャッシュに保存
	s.cache.Set(cacheKey, post)

	logger.Info("successfully got post", "id", id, "title", post.Title)
	return post, nil
}

// CreatePost 記事を作成（キャッシュ無効化）
func (s *BlogService) CreatePost(ctx context.Context, req *model.PostRequest) (*model.Post, error) {
	logger.Info("creating post", "title", req.Title)

	if err := s.validatePostRequest(req); err != nil {
		logger.Error("invalid post request", "error", err)
		return nil, fmt.Errorf("invalid post request: %w", err)
	}

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

	// キャッシュを無効化
	s.invalidateListCache()

	logger.Info("successfully created post", "id", post.ID, "title", post.Title)
	return post, nil
}

// UpdatePost 記事を更新（キャッシュ無効化）
func (s *BlogService) UpdatePost(ctx context.Context, id int, req *model.PostRequest) (*model.Post, error) {
	logger.Info("updating post", "id", id, "title", req.Title)

	if id <= 0 {
		return nil, fmt.Errorf("invalid post ID: %d", id)
	}

	if err := s.validatePostRequest(req); err != nil {
		logger.Error("invalid post request", "error", err)
		return nil, fmt.Errorf("invalid post request: %w", err)
	}

	existingPost, err := s.repo.GetPost(ctx, id)
	if err != nil {
		logger.Error("post not found for update", "id", id, "error", err)
		return nil, fmt.Errorf("post not found: %w", err)
	}

	existingPost.Title = req.Title
	existingPost.Content = req.Content

	if err := s.repo.UpdatePost(ctx, id, existingPost); err != nil {
		logger.Error("failed to update post", "id", id, "error", err)
		return nil, fmt.Errorf("failed to update post: %w", err)
	}

	// キャッシュを無効化
	s.invalidatePostCache(id)
	s.invalidateListCache()

	logger.Info("successfully updated post", "id", id, "title", existingPost.Title)
	return existingPost, nil
}

// DeletePost 記事を削除（キャッシュ無効化）
func (s *BlogService) DeletePost(ctx context.Context, id int) error {
	logger.Info("deleting post", "id", id)

	if id <= 0 {
		return fmt.Errorf("invalid post ID: %d", id)
	}

	_, err := s.repo.GetPost(ctx, id)
	if err != nil {
		logger.Error("post not found for deletion", "id", id, "error", err)
		return fmt.Errorf("post not found: %w", err)
	}

	if err := s.repo.DeletePost(ctx, id); err != nil {
		logger.Error("failed to delete post", "id", id, "error", err)
		return fmt.Errorf("failed to delete post: %w", err)
	}

	// キャッシュを無効化
	s.invalidatePostCache(id)
	s.invalidateListCache()

	logger.Info("successfully deleted post", "id", id)
	return nil
}

// invalidatePostCache 特定記事のキャッシュを無効化
func (s *BlogService) invalidatePostCache(id int) {
	cacheKey := fmt.Sprintf("post:%d", id)
	s.cache.Delete(cacheKey)
	logger.Debug("invalidated post cache", "id", id)
}

// invalidateListCache 記事一覧のキャッシュを無効化
func (s *BlogService) invalidateListCache() {
	cacheKey := "posts:list"
	s.cache.Delete(cacheKey)
	logger.Debug("invalidated list cache")
}

// GetCacheStats キャッシュ統計を取得
func (s *BlogService) GetCacheStats() map[string]interface{} {
	return map[string]interface{}{
		"size": s.cache.Size(),
		"ttl":  s.cache.TTL(),
	}
}
```

### 5. 接続プールの最適化

**目的**: AWS SDKの接続を効率化

**ファイル**: `lambda/internal/config/aws.go`

```go
package config

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// AWSConfig AWS設定の最適化
type AWSConfig struct {
	S3Client *s3.Client
}

// NewAWSConfig 最適化されたAWS設定を作成
func NewAWSConfig() (*AWSConfig, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRetryer(func() aws.Retryer {
			return aws.NopRetryer{} // LocalStack用にリトライを無効化
		}),
		config.WithHTTPClient(&http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true // LocalStack用
	})

	return &AWSConfig{
		S3Client: s3Client,
	}, nil
}
```

### 6. メトリクス収集の追加

**目的**: パフォーマンス監視

**ファイル**: `lambda/internal/metrics/metrics.go`

```go
package metrics

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"lambda/pkg/logger"
)

// Metrics メトリクス収集
type Metrics struct {
	client *cloudwatch.Client
}

// NewMetrics メトリクスのコンストラクタ
func NewMetrics() *Metrics {
	cfg, _ := config.LoadDefaultConfig(context.Background())
	return &Metrics{
		client: cloudwatch.NewFromConfig(cfg),
	}
}

// RecordLatency レイテンシを記録
func (m *Metrics) RecordLatency(operation string, duration time.Duration) {
	logger.Info("recording latency",
		"operation", operation,
		"duration_ms", duration.Milliseconds())

	// CloudWatchメトリクスに送信（本番環境）
	// LocalStackではログ出力のみ
}

// RecordCacheHit キャッシュヒットを記録
func (m *Metrics) RecordCacheHit(key string) {
	logger.Info("cache hit", "key", key)
}

// RecordCacheMiss キャッシュミスを記録
func (m *Metrics) RecordCacheMiss(key string) {
	logger.Info("cache miss", "key", key)
}

// RecordError エラーを記録
func (m *Metrics) RecordError(operation string, err error) {
	logger.Error("operation error",
		"operation", operation,
		"error", err.Error())
}
```

### 7. 最適化されたメイン関数

**目的**: 初期化処理の最適化

**ファイル**: `lambda/cmd/blog/main.go`（更新）

```go
package main

import (
	"context"
	"os"

	"github.com/aws/aws-lambda-go/lambda"

	"lambda/internal/config"
	"lambda/internal/handler"
	"lambda/internal/repository"
	"lambda/internal/service"
	"lambda/pkg/logger"
)

func main() {
	logger.Info("starting optimized blog API")

	// 環境変数の取得
	bucket := os.Getenv("POSTS_BUCKET")
	if bucket == "" {
		logger.Error("POSTS_BUCKET environment variable is required")
		os.Exit(1)
	}

	// AWS設定の初期化（最適化版）
	awsConfig, err := config.NewAWSConfig()
	if err != nil {
		logger.Error("failed to initialize AWS config", "error", err)
		os.Exit(1)
	}

	// リポジトリの初期化（最適化版）
	repo, err := repository.NewS3RepositoryWithClient(bucket, awsConfig.S3Client)
	if err != nil {
		logger.Error("failed to initialize repository", "error", err)
		os.Exit(1)
	}

	// サービスの初期化（キャッシュ対応）
	blogService := service.NewBlogService(repo)

	// ハンドラーの初期化
	blogHandler := handler.NewBlogHandler(blogService)

	logger.Info("blog API initialized successfully")

	// Lambda関数の開始
	lambda.Start(blogHandler.HandleRequest)
}
```

## 動作確認

### パフォーマンス測定

```bash
# Cold Start時間の測定
echo "Measuring Cold Start time..."
time curl -s "${BASE}/posts" > /dev/null

# 2回目以降の応答時間（キャッシュ効果）
echo "Measuring cached response time..."
time curl -s "${BASE}/posts" > /dev/null

# 複数回実行での平均時間
echo "Measuring average response time..."
for i in {1..10}; do
  echo "Request $i:"
  time curl -s "${BASE}/posts" > /dev/null
done
```

### キャッシュ動作確認

```bash
# 記事作成
curl -s -X POST \
  -H "Content-Type: application/json" \
  -d '{"title":"Performance Test","content":"Testing cache performance"}' \
  "${BASE}/posts" | jq .

# 記事取得（キャッシュヒット）
curl -s "${BASE}/posts/1" | jq .

# 記事一覧取得（キャッシュヒット）
curl -s "${BASE}/posts" | jq .
```

### 期待結果

- **Cold Start時間**: 2.5秒 → 1.5秒以下（40%改善）
- **API応答時間**: 800ms → 200ms以下（75%改善）
- **メモリ使用量**: 128MB → 256MB（安定性向上）
- **キャッシュヒット率**: 0% → 80%以上

## トラブルシューティング

### メモリ不足エラー

**症状**: Lambda関数でメモリ不足エラー

**原因**: キャッシュサイズが大きすぎる

**解決策**:

```go
// キャッシュサイズの制限
cache := cache.NewMemoryCacheWithLimit(5*time.Minute, 100) // 最大100アイテム
```

### キャッシュが効かない

**症状**: 応答時間が改善されない

**原因**: キャッシュキーの重複やTTL設定

**解決策**:

```bash
# ログでキャッシュ動作を確認
awslocal logs tail "/aws/lambda/BlogApi" --follow | grep cache
```

### 接続タイムアウト

**症状**: S3アクセスでタイムアウト

**原因**: 接続プール設定の問題

**解決策**:

```go
// タイムアウト設定の調整
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()
```

## 次のステップ

パフォーマンス最適化が完了したら、[テスト追加](../refactoring/04-testing.md)に進んでください。

**完了確認**:

- [ ] Lambda設定が最適化されている
- [ ] メモリキャッシュが実装されている
- [ ] S3アクセスが最適化されている
- [ ] メトリクス収集が実装されている
- [ ] パフォーマンスが改善されている

---

> **💡 ヒント**: パフォーマンス最適化は段階的に進めることが重要です。各最適化の効果を測定しながら進めてください。
