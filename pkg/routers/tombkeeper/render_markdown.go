package tombkeeper

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/render"
)

// RenderMarkdown 从自包含快照确定性渲染一篇博文，不执行 I/O 或读取全局配置。
func RenderMarkdown(postID int64, content ContentSnapshot, serverBaseURL string) (string, error) {
	post, ok := content.Posts[postID]
	if !ok {
		return "", fmt.Errorf("post %d is missing from content snapshot", postID)
	}
	return renderSnapshotPost(post, content, serverBaseURL, 0), nil
}

func renderSnapshotPost(post Post, content ContentSnapshot, serverBaseURL string, depth int) string {
	h5 := snapshotH5Images(post, content, depth)
	body, inlineQuotes := renderSnapshotLinks(
		escapeMarkdown(post.Text), post.Links, content, serverBaseURL, depth, h5,
	)

	sections := []string{body}
	sections = append(sections, snapshotPostImages(post, content)...)
	if video, rawURL := videoLink(post.Links); video != "" && !strings.Contains(body, rawURL) {
		sections = append(sections, video)
	}
	sections = append(sections, snapshotH5Embeds(h5)...)
	sections = append(sections, inlineQuotes...)

	if depth == 0 && post.RetweetPostID != 0 {
		if original, ok := content.Posts[post.RetweetPostID]; ok {
			quote := snapshotQuoteBody(original, content, serverBaseURL)
			sections = append(sections, quoteBlock("转发 @"+original.ScreenName, quote))
		}
	}
	return strings.TrimRight(md.Join(sections...), "\n")
}

func snapshotPostImages(post Post, content ContentSnapshot) []string {
	out := make([]string, 0, len(post.Pics))
	sourceURL := WeiboPostURL(post.AuthorID, post.Bid, strconv.FormatInt(post.ID, 10))
	for index, rawImage := range post.Pics {
		if strings.HasPrefix(rawImage, "http") && !imageURLAllowed(rawImage) {
			out = append(out, md.Image(fmt.Sprintf("微博图片 %d", index+1), rawImage))
			continue
		}
		asset, ok := content.Images[picIDOf(rawImage)]
		if ok && asset.Status == ObjectStatusOK {
			if uri, err := asset.URI(); err == nil {
				out = append(out, md.Image(fmt.Sprintf("微博图片 %d", index+1), uri))
				continue
			}
		}
		out = append(out, imageFailedNotice(index+1, sourceURL))
	}
	return out
}

type snapshotImage struct {
	number int
	url    string
}

func snapshotH5Images(post Post, content ContentSnapshot, depth int) map[string][]snapshotImage {
	images := make(map[string][]snapshotImage)
	if depth != 0 {
		return images
	}
	number := 0
	for _, link := range post.Links {
		if !isViewPic(link) {
			continue
		}
		for _, id := range post.H5ImageIDsByURL[link.LongURL] {
			asset, ok := content.Images[id]
			if !ok || asset.Status != ObjectStatusOK {
				continue
			}
			uri, err := asset.URI()
			if err != nil {
				continue
			}
			number++
			images[link.LongURL] = append(images[link.LongURL], snapshotImage{number: number, url: uri})
		}
	}
	return images
}

func snapshotH5Embeds(images map[string][]snapshotImage) []string {
	var ordered []snapshotImage
	for _, entries := range images {
		ordered = append(ordered, entries...)
	}
	// map 无序，按已分配的图片编号恢复展示顺序。
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].number < ordered[j].number })
	out := make([]string, 0, len(ordered))
	for _, image := range ordered {
		out = append(out, md.Image(fmt.Sprintf("微博图片 %d", image.number), image.url))
	}
	return out
}

func renderSnapshotLinks(text string, links []PostLink, content ContentSnapshot, serverBaseURL string,
	depth int, h5 map[string][]snapshotImage,
) (string, []string) {
	byShort := make(map[string]PostLink, len(links))
	for _, link := range links {
		if link.ShortURL != "" {
			byShort[link.ShortURL] = link
		}
	}
	var quotes []string
	quoteNumber := 0
	text = tcnRe.ReplaceAllStringFunc(text, func(token string) string {
		link, ok := byShort[token]
		if !ok {
			return token
		}
		if isViewPic(link) {
			if images := h5[link.LongURL]; len(images) > 0 {
				return fmt.Sprintf("[微博图片 %d](%s)", images[0].number, images[0].url)
			}
			return fmt.Sprintf("[查看图片|原始链接](%s)", link.LongURL)
		}
		if depth == 0 && isWeiboTextLink(link) {
			_, bid := parseWeiboLong(link.LongURL)
			mid, err := BidToMid(bid)
			if err == nil {
				id, parseErr := strconv.ParseInt(mid, 10, 64)
				if target, exists := content.Posts[id]; parseErr == nil && exists {
					quoteNumber++
					body := snapshotQuoteBody(target, content, serverBaseURL)
					quotes = append(quotes,
						quoteBlock(fmt.Sprintf("微博正文%d @%s", quoteNumber, target.ScreenName), body))
					archiveURL := render.BuildArchiveLink(serverBaseURL, link.LongURL)
					return fmt.Sprintf("[微博正文%d](%s)", quoteNumber, archiveURL)
				}
			}
		}
		return plainLink(link)
	})
	return text, quotes
}

func snapshotQuoteBody(post Post, content ContentSnapshot, serverBaseURL string) string {
	body := renderSnapshotPost(post, content, serverBaseURL, 1)
	if post.PublishedAt.IsZero() {
		return body
	}
	return md.Join(body, retweetTimeLine(post.PublishedAt))
}

// 微博正文是纯文本；#话题#、@用户和词内下划线保持字面值。
var mdInlineEscaper = strings.NewReplacer(
	`\`, `\\`,
	"`", "\\`",
	`*`, `\*`,
	`[`, `\[`,
	`]`, `\]`,
	`<`, `\<`,
	`~`, `\~`,
	`|`, `\|`, // GFM table cell delimiter
)

// 只转义行首块标记，3.14 等小数保持字面值。
var (
	lineStartRe        = regexp.MustCompile(`^(\s*)([#>\-+])`)
	lineStartOrderedRe = regexp.MustCompile(`^(\s*)(\d{1,9})([.)])(\s|$)`)
)

func escapeMarkdown(s string) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		ln = mdInlineEscaper.Replace(ln)
		ln = lineStartRe.ReplaceAllString(ln, "$1\\$2")
		ln = lineStartOrderedRe.ReplaceAllString(ln, "$1$2\\$3$4")
		lines[i] = ln
	}
	return strings.Join(lines, "\n")
}

// makeTitle 折叠空白后取正文前 10 个 rune 作为 RSS 标题。
func makeTitle(text string) string {
	t := strings.Join(strings.Fields(text), " ")
	r := []rune(t)
	if len(r) > 10 {
		r = r[:10]
	}
	return string(r)
}

// 微博历史时期中国大陆无夏令时；固定 UTC+8 以保持纯渲染确定性。
var beijing = time.FixedZone("Beijing", 8*3600)

func retweetTimeLine(t time.Time) string {
	return t.In(beijing).Format("2006 年 01 月 02 日 15:04")
}

func quoteBlock(header, body string) string {
	content := header
	if body != "" {
		content += "\n\n" + body
	}
	return md.Quote(content)
}

func imageFailedNotice(number int, sourceURL string) string {
	return md.Quote(fmt.Sprintf("微博图片 %d 下载失败，请前往 [源微博](%s) 查看", number, sourceURL))
}

func splitPics(raw string) []string {
	var pics []string
	seen := make(map[string]struct{})
	for pic := range strings.SplitSeq(raw, ",") {
		pic = strings.TrimSpace(pic)
		if pic == "" {
			continue
		}
		id := picIDOf(pic)
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		pics = append(pics, pic)
	}
	return pics
}

func videoLink(links []PostLink) (markdown, rawURL string) {
	for _, link := range links {
		if strings.Contains(link.URLTitle, "微博视频") || strings.Contains(link.LongURL, "video.weibo.com") {
			return fmt.Sprintf("[微博视频](%s)", link.LongURL), link.LongURL
		}
	}
	return "", ""
}
