package apiModels

// XMLInitalState represents json from xml <body><script id="js-initialData" type="text/json">...</script></body>
type XmlInitalData struct {
	InitialState struct {
		Entities struct {
			Answers map[string]XmlAnswer `json:"answers"`
		} `json:"entities"`
	} `json:"initialState"`
}

type XmlAnswer struct {
	ID          int      `json:"id"`
	Content     string   `json:"content"`
	CreatedTime int      `json:"createdTime"`
	Author      Author   `json:"author"`
	Question    Question `json:"question"`
}
