package dto

type CreateGroupReq struct {
	Name     string   `json:"name" form:"name"`
	Avatar   string   `json:"avatar" form:"avatar" binding:"omitempty"`
	OwnerID  uint64   `json:"owner_id" form:"owner_id"`
	JoinType byte     `json:"join_type" form:"join_type" binding:"omitempty"`
	Members  []uint64 `json:"members" form:"members" binding:"omitempty"` // 群成员 ID
}

type CreateGroupResp struct {
	GroupKeyID int64  `json:"group_key_id"`
	GroupID    uint64 `json:"group_id"`
}
