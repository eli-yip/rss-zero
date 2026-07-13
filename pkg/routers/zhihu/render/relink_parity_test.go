package render

import (
	"regexp"
	"testing"

	"github.com/lib/pq"

	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
)

// autocorrect-disable -- 本文件字面量是换链的输入 fixture 与期望输出，必须逐字节一致，禁止 autocorrect 改动

// 本测试记录一次「把 relinkImages 从逐链 whole-body 替换改成单趟 ReplaceAllStringFunc」的
// 性能改动被否决的原因：两种做法在 alt 含 `]` 的图片链接上发散，故生产代码保留逐链做法。
//
// 根因：生产 replaceImageLink 用 `!\[[^\]]*\]` 匹配 alt（遇第一个 `]` 即止），与 FindImageLinks /
// imageLinkRe 找链接用的 `.*?` 不对称。当某图片链接的 alt 内含 `]`（如 `![a]b](url)`）时：
// FindImageLinks 仍能取到 url、进入换链循环，但 replaceImageLink 的正则匹配不到这条链接，于是
// 原样放过；而单趟 ReplaceAllStringFunc 复用 imageLinkRe 的 `.*?`，会把这条链接也换掉——两者
// 输出发散。zhihu figure 规则产出的 alt 恒为空（`![](url)`），故 golden 不受影响，但该发散是
// 真实的正确性差异，不是纯合成，故改动整体回退。
//
// oldReplaceImageLink / oldRelink 是当前生产实现的逐字复制（对照基线）；newRelinkRejected 是被
// 否决的单趟候选实现。测试断言：生产 relinkImages 与 oldRelink 恒等；两种做法仅在 alt 含 `]`
// 的用例上发散、其余对抗性用例逐字节一致。

// oldReplaceImageLink 是生产 internal.go replaceImageLink 的逐字复制。
func oldReplaceImageLink(text, name, from, to string) string {
	re := regexp.MustCompile(`!\[[^\]]*\]\(` + regexp.QuoteMeta(from) + `\)`)
	return re.ReplaceAllString(text, `![`+name+`](`+to+`)`)
}

// oldRelink 是生产 snapshot.go relinkImages 的逐字复制：FindImageLinks 取全量原链、逐个对
// whole body 做 QuoteMeta 正则替换。
func oldRelink(text string, content ContentSnapshot) string {
	for _, link := range FindImageLinks(text) {
		object, ok := content.Objects[URLToID(link)]
		if !ok || len(object.StorageProvider) == 0 {
			continue // 降级：保留原始链接
		}
		objectURL := object.StorageProvider[0] + "/" + object.ObjectKey
		text = oldReplaceImageLink(text, object.ObjectKey, link, objectURL)
	}
	return text
}

// newRelinkRejected 是被否决的单趟候选：用 imageLinkRe 一趟扫过正文、逐个 ![alt](url) 就地换链。
func newRelinkRejected(text string, content ContentSnapshot) string {
	return imageLinkRe.ReplaceAllStringFunc(text, func(match string) string {
		sub := imageLinkRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		object, ok := content.Objects[URLToID(sub[1])]
		if !ok || len(object.StorageProvider) == 0 {
			return match // 降级：保留原始链接
		}
		return "![" + object.ObjectKey + "](" + object.StorageProvider[0] + "/" + object.ObjectKey + ")"
	})
}

// snapWithObjects 为给定的一组「存在的」url 造图片对象快照；未列入的 url 会走降级分支。
func snapWithObjects(urls ...string) ContentSnapshot {
	objs := make(map[int]zhihuDB.Object, len(urls))
	for _, u := range urls {
		id := URLToID(u)
		objs[id] = zhihuDB.Object{
			ID:              id,
			ObjectKey:       "zhihu/" + itoa(id) + ".jpg",
			StorageProvider: pq.StringArray{testProvider},
		}
	}
	return ContentSnapshot{Objects: objs}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// TestRelinkParityAdversarial 对每个对抗性正文，跑生产 relinkImages、其逐字复制 oldRelink、以及
// 被否决的单趟候选 newRelinkRejected：断言生产与 oldRelink 恒等，且逐链 vs 单趟仅在 alt 含 `]`
// 的用例上发散（wantDiverge=true），其余用例逐字节一致。
func TestRelinkParityAdversarial(t *testing.T) {
	const (
		u1 = "https://pic.zhihu.com/v2-one.jpg"
		u2 = "https://pic.zhihu.com/v2-two.jpg"
		ua = "http://x.com/a"
		ub = "http://x.com/ab" // ua 是 ub 的子串
	)

	cases := []struct {
		name        string
		body        string
		present     []string // 造对象的 url，其余走降级
		wantDiverge bool     // 逐链(old) 与单趟(new) 是否发散
	}{
		{"single-empty-alt", `![](` + u1 + `)`, []string{u1}, false},
		{"same-url-twice-empty-alt", `![](` + u1 + `) ![](` + u1 + `)`, []string{u1}, false},
		{"same-url-twice-diff-alt", `![x](` + u1 + `) ![y](` + u1 + `)`, []string{u1}, false},
		{"duplicate-identical", `![z](` + u1 + `)` + "\n" + `![z](` + u1 + `)`, []string{u1}, false},
		{"substring-urls", `![](` + ua + `) ![](` + ub + `)`, []string{ua, ub}, false},
		{"substring-urls-only-short-present", `![](` + ua + `) ![](` + ub + `)`, []string{ua}, false},
		{"substring-urls-only-long-present", `![](` + ua + `) ![](` + ub + `)`, []string{ub}, false},
		{"url-in-plain-text", `see ` + ua + ` here ![](` + ua + `)`, []string{ua}, false},
		{"alt-regex-special-no-bracket", `![a.*+$^(b](` + u1 + `)`, []string{u1}, false},
		{"alt-with-open-bracket", `![a[b](` + u1 + `)`, []string{u1}, false},
		// alt 含 `]`：逐链放过、单趟换掉——发散。这是改动被否决的原因。
		{"alt-with-close-bracket", `![a]b](` + u1 + `)`, []string{u1}, true},
		{"alt-with-escaped-bracket", `![a\]b](` + u1 + `)`, []string{u1}, true},
		{"missing-object-degrade", `![](` + u1 + `) ![](` + u2 + `)`, []string{u1}, false},
		{"url-with-regex-special", `![](http://x.com/a.b?q=1+2*3)`, []string{"http://x.com/a.b?q=1+2*3"}, false},
		{"no-image-links", `just text (with parens) and ![incomplete alt`, nil, false},
		{"consecutive-no-space", `![](` + u1 + `)![](` + u2 + `)`, []string{u1, u2}, false},
		{"cjk-alt", `![图片说明](` + u1 + `)`, []string{u1}, false}, // autocorrect-disable-line
		{"mixed", `开头 ![](` + u1 + `) 中间 ` + ua + ` 文字 ![alt.text](` + ub + `) 结尾`, []string{u1, ub}, false}, // autocorrect-disable-line
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap := snapWithObjects(tc.present...)
			prod := relinkImages(tc.body, snap)
			old := oldRelink(tc.body, snap)
			newCand := newRelinkRejected(tc.body, snap)

			// oldRelink 必须逐字节复现生产 relinkImages（对照基线可信）。
			if prod != old {
				t.Fatalf("oldRelink 未复现生产实现\n body: %q\n prod: %q\n  old: %q", tc.body, prod, old)
			}

			if tc.wantDiverge {
				// 记录发散：单趟候选把 alt 含 `]` 的链接也换掉，生产逐链做法放过。
				if old == newCand {
					t.Fatalf("预期发散但一致，用例失去意义\n body: %q\n  out: %q", tc.body, old)
				}
				t.Logf("已记录发散\n body: %q\n 逐链(生产): %q\n 单趟(否决): %q", tc.body, old, newCand)
			} else {
				if old != newCand {
					t.Fatalf("换链发散（不应发散）\n body: %q\n 逐链: %q\n 单趟: %q", tc.body, old, newCand)
				}
			}
		})
	}
}

// autocorrect-enable
