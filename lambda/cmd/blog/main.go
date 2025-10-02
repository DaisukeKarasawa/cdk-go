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

func jsonOK(v interface{}) events.APIGatewayProxyResponse {
	b, _ := json.Marshal(v)
	return events.APIGatewayProxyResponse{StatusCode: 200, Body: string(b), Headers: map[string]string{"Content-Type": "application/json"}}
}

func errorJSON(code int, msg string) (events.APIGatewayProxyResponse, error) {
	b, _ := json.Marshal(map[string]string{"error": msg})
	return events.APIGatewayProxyResponse{StatusCode: code, Body: string(b), Headers: map[string]string{"Content-Type": "application/json"}}, nil
}

func handle(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	path := req.Path
	method := req.HTTPMethod

	if method == http.MethodGet && path == "/posts" {
		// 一覧
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

			var p Post
			b, _ := io.ReadAll(po.Body)
			_ = po.Body.Close()
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
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: string(b), Headers: map[string]string{"Content-Type": "application/json"}}, nil
	}

	if method == http.MethodPost && path == "/posts" {
		var p Post
		if err := json.Unmarshal([]byte(req.Body), &p); err != nil || p.ID == 0 {
			return errorJSON(400, "invalid body: require id,title,content")
		}

		key := fmt.Sprintf("posts/%d.json", p.ID)
		b, _ := json.Marshal(p)
		ct := "application/json"
		_, err := s3Client.PutObject(ctx, &s3.PutObjectInput{Bucket: &bucket, Key: &key, Body: bytes.NewReader(b), ContentType: &ct})
		if err != nil {
			return errorJSON(500, "create failed")
		}
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: string(b), Headers: map[string]string{"Content-Type": "application/json"}}, nil
	}

	if method == http.MethodPut && strings.HasPrefix(path, "/posts/") {
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

func main() {
	lambda.Start(handle)
}
