package common

const (
	TypeZhihuAnswer = iota
	TypeZhihuArticle
	TypeZhihuPin
)

func ZhihuTypeToString(subType int) (ts string) {
	switch subType {
	case TypeZhihuAnswer:
		ts = "answer"
	case TypeZhihuArticle:
		ts = "article"
	case TypeZhihuPin:
		ts = "pin"
	}

	return ts
}
