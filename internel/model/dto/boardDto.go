package dto

import "time"

type BoardRequest struct {
	UserID   uint64 `json:"user_id"`
	BoardID  string `json:"board_id" binding:"omitempty"`
	Title    string `json:"title" binding:"omitempty"`
	IsShared bool   `json:"is_shared" binding:"omitempty"`
}

type BoardResponse struct {
	BoardID   string    `json:"board_id"`
	UserID    uint64    `json:"user_id"`
	Title     string    `json:"title"`
	IsShared  bool      `json:"is_shared"`
	ShareCode string    `json:"share_code"`
	Snapshot  []byte    `json:"snapshot"`
	CreatedAt time.Time `json:"create_at"`
}
