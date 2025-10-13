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

// S3ベースの記事リポジトリ
type S3Repository struct {
	client *s3.Client
	bucket string
}

// S3リポジトリのコンストラクタ
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

// 404エラーの判定
func isNotFoundError(err error) bool {
	var notFound *types.NoSuchKey
	return err != nil && errors.As(err, &notFound)
}

// S3キーから記事を取得
func (r *S3Repository) getPostByKey(ctx context.Context, key string) (*model.Post, error) {
	out, err := r.client.GetObject(ctx, &s3.GetObjectInput){
		Bucket: &r.bucket,
		Key:    &key,
	}
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


// 記事をS3に保存
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

// 記事一覧を取得
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

// 指定IDの記事を取得
func (r *S3Repository) GetPost(ctx context.Context, id int) (*model.Post, error) {
	logger.Info("getting post from S3", "id", id, "bucket", r.bucket)

	key := fmt.Sprintf("posts/%d.json", id)
	return r.getPostByKey(ctx, key)
}

// 記事を作成
func (r *S3Repository) CreatePost(ctx context.Context, post *model.Post) error {
	logger.Info("creating post in S3", "id", post.ID, "title", post.Title)

	key := fmt.Sprintf("posts/%d.json", post.ID)
	return r.savePost(ctx, key, post)
}

// 記事を更新
func (r *S3Repository) UpdatePost(ctx context.Context, id int, post *model.Post) error {
	logger.Info("updating post in S3", "id", id, "title", post.Title)

	key := fmt.Sprintf("posts/%d.json", id)
	post.ID = id // URLのIDを優先
	return r.savePost(ctx, key, post)
}

// 記事を削除
func (r *S3Repository) DeletePost(ctx context.Context, id int) error {
	logger.Info("deleting post from S3", "id", id)


	key := fmt.Sprintf("posts/%d.json", id)
	_, err := r.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &r.bucket,
		Key:    &key,
	})
	if err != nil {
		logger.Error("failed to delete post", "id", id, "error", err)
	}

	logger.Info("successfully deleted post", "id", id)
	return nil
}
