package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type Post struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

func handleRequest(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	path := req.Path
	method := req.HTTPMethod

	// 簡易ルーティング
	if method == http.MethodGet && path == "/posts" {
		// 仮置き
		posts := []Post{
			{
				ID:      "hello",
				Title:   "Hello",
				Content: "Hello from LocalStack",
			},
		}

		b, _ := json.Marshal(posts)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: string(b), Headers: map[string]string{"Content-Type": "application/json"}}, nil
	}
	if method == http.MethodGet && strings.HasPrefix(path, "/posts/") {
		id := strings.TrimPrefix(path, "/posts/")
		content := fmt.Sprintf("# %s\n\nThis is a mock article.", id)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: content, Headers: map[string]string{"Content-Type": "text/markdown; charset=utf-8"}}, nil
	}

	return events.APIGatewayProxyResponse{StatusCode: 404, Body: "not found"}, nil
}

func main() {
	_ = os.Setenv("AWS_SDK_LOAD_CONFIG", "1")
	lambda.Start(handleRequest)
}
