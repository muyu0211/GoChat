package dto

type CreateGroupReq struct {
	Name     string   `json:"name"`
	Avatar   string   `json:"avatar" binding:"omitempty"`
	OwnerID  uint64   `json:"owner_id"`
	JoinType byte     `json:"join_type"`
	Members  []uint64 `json:"members" binding:"omitempty"` // 群成员 ID
}

type CreateGroupResp struct {
	GroupID uint64 `json:"group_id"`
}
