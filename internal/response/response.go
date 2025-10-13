package response

import (
	"encoding/json"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
)

// 成功レスポンスを生成
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

// 作成成功レスポンス
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

// 削除成功レスポンス
func NoContent() events.APIGatewayProxyResponse {
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusNoContent,
		Body:       "",
	}
}

// エラーレスポンス
func errorResponse(code int, message string) events.APIGatewayProxyResponse {
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

// バッドリクエストエラー
func BadRequest(message string) events.APIGatewayProxyResponse {
	return errorResponse(http.StatusBadRequest, message)
}

// リソース未発見エラー
func NotFound(message string) events.APIGatewayProxyResponse {
	return errorResponse(http.StatusNotFound, message)
}

// 内部サーバーエラー
func InternalServerError(message string) events.APIGatewayProxyResponse {
	return errorResponse(http.StatusInternalServerError, message)
}
