package md

func Image(alt, url string) (text string) { return `![` + alt + `](` + url + `)` }
