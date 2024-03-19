package render

func ExtractExcerpt(text string) (excerpt string) {
	if len(text) > 100 {
		return text[:100]
	}
	return text
}
