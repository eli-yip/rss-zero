package common

type (
	Cookie struct {
		Value    string `json:"value"`
		ExpireAt any    `json:"expire_at"`
	}
)
