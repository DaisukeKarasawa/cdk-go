package model

import "time"

// ブログ記事のデータモデル
type Post struct {
  ID        int       `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
  UpdatedAt time.Time `json:"updated_at"`
}

// 記事作成・更新リクエスト
type PostRequest struct {
  Title   string `json:"title"`
	Content string `json:"content"`
}

// 記事一覧レスポンス
type PostListResponse struct {
	Posts []Post `json:"posts"`
	Total int    `json:"total"`
}
