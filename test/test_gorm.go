package test

type PushPayLoad struct {
	Msg          []byte `json:"msg_data"`
	ReceiverID   uint64 `json:"receiver_id"`
	TargetServer string `json:"target_server"`
}

func test() {

}
