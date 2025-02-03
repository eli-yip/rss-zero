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

func ZhihuTypeToLinkType(subType int) (u string) {
	switch subType {
	case TypeZhihuAnswer:
		u = "answers"
	case TypeZhihuArticle:
		u = "posts"
	case TypeZhihuPin:
		u = "pins"
	}

	return u
}
