package parse

import "github.com/pemistahl/lingua-go"

func (s *ParseService) detectLanguage(text string) (language lingua.Language, exists bool) {
	return s.languageDetector.DetectLanguageOf(text)
}
