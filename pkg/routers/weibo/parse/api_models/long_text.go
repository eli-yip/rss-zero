package apiModels

type LongTextApiResp struct {
	Data struct {
		LongTextContent string `json:"longTextContent"`
	} `json:"data"`
}
