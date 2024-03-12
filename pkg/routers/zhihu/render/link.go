package render

import "fmt"

func GenerateAnswerLink(questionID, answerID int) string {
	return fmt.Sprintf("https://www.zhihu.com/question/%d/answer/%d", questionID, answerID)
}

func GenerateArticleLink(articleID int) string {
	return fmt.Sprintf("https://zhuanlan.zhihu.com/p/%d", articleID)
}

func GeneratePinLink(pinID int) string {
	return fmt.Sprintf("https://www.zhihu.com/pin/%d", pinID)
}
