package dto

type CreateGroupReq struct {
	Name    string   `json:"name"`
	Avatar  string   `json:"avatar"`
	Members []uint64 `json:"members"` // 群成员 ID
}

type CreateGroupResp struct {
	GroupID uint64 `json:"group_id"`
}
