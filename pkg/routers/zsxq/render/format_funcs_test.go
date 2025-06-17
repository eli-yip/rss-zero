package render

import (
	"encoding/json"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testCase struct {
	input  string
	expect string
}

func TestMain(t *testing.T) {
	testText := []testCase{
		{`34234243[test text](https://rss-zero.com/123456)`, `34234243[test text](https://rss-zero.com/123456)`},
		{`34234243[test text](https://rss-zero.com/1234 56)`, `34234243[test text](https://rss-zero.com/1234%2056)`},
		{`34234243[test text](https://rss-zero.com/123456)123[test text](https://rss-zero.com/1234 56)`, `34234243[test text](https://rss-zero.com/123456)123[test text](https://rss-zero.com/1234%2056)`},
	}

	assert := assert.New(t)
	for _, tc := range testText {
		output, err := escapeLinkPath(tc.input)
		assert.Nil(err)
		assert.Equal(tc.expect, output)
	}
}

func TestFormatBold(t *testing.T) {
	assert := assert.New(t)

	// autocorrect: false
	expect := "康波周期律及其他周期。\n\n\\#成长小谈 \\#认知与命运 \n\n之前提过很多周期律的问题，我们整理一下已有的重要周期理论。\n\n朋友们感兴趣的可以去寻找资料，详细了解。\n\n康波周期：\n\n康波周期（康德拉季耶夫长波理论）是描述资本主义经济体长期波动（**50-60年**）的核心理论，由俄国经济学家 **尼古拉·康德拉季耶夫** 于1925年提出。其本质是**技术革命驱动的系统性经济变迁周期**。\n\n从技术萌芽 → 产业爆发 → 成熟过剩 → 停滞衰退，推动经济从繁荣滑向萧条。\n\n之前已经有过四波周期：\n\n第一波1780s-1840s         水力机械+纺织机械化蒸汽机、纺织厂；\n第二波1840s-1890s         铁路+钢铁洲际铁路、炼钢法；\n第三波1890s-1940s         电力+重化工电网、汽车流水线；\n**第四波****1940s-1990s        ****石化+电子工业**计算机、喷气式客机、核能；\n**第五波1990s-2040s        信息技术+互联网**个人电脑、智能手机、云计算\n\n\n**四阶段划分：繁荣、衰退、萧条、回升**\n**繁荣期（Spring）**:\n新技术爆发式增长（如1990-2000年互联网）\n生产效率暴涨 → 低通胀高增长 → 股市长牛\n**衰退期（Summer）**:\n技术红利衰减（如2000年后互联网泡沫破裂）\n资本过度投机 → 资产泡沫破裂（如2008年金融危机）\n**标志**：实物资产（原油/矿产）价格暴涨后崩溃\n**萧条期（Autumn）**:\n旧技术产能彻底过剩（如2015年后全球增长乏力）\n企业利润萎缩 → 失业率上升 → 社会矛盾激化\n**当前状态**：第五波康波（信息技术）的萧条期（2020-2035？）\n**回升期（Winter）**:\n下一代技术革命萌芽（如当前AI/可控核聚变）\n资本向新领域试探 → 旧产能出清完成。\n\n康波周期的问题是样本不够多，归因的情况过于单一，以及未来新技术可能导致周期大幅度缩短。\n\n其他可以了解的周期：\n\n**朱格拉周期（Juglar Cycle）**是经济学中重要的**中期经济波动理论**，由法国经济学家 **克莱门特·朱格拉（Clément Juglar）** 于1862年提出，核心揭示 **企业固定资产更替驱动的7-11年经济周期**。企业为维持竞争力，必须周期性更新生产设备（厂房、机床、机械等），这一行为形成闭环：**设备老化 → 技术升级需求 → 资本开支上升 → 产能扩张 → 供给过剩 → 盈利下滑 → 投资收缩 → 萧条出清 → 等待新设备需求**\n\n**库兹涅茨周期（Kuznets Cycle）** 是经济学家 **西蒙·库兹涅茨（Simon Kuznets）** 在1930年代提出的**15-25年经济长波理论**，核心聚焦 **人口迁移驱动的建筑与房地产投资周期**。其本质揭示了**城镇化进程中基础设施建设的波浪式发展规律**。\n\n**熊彼特周期（Schumpeterian Cycle）**并非传统时间维度的周期理论，而是由奥地利经济学家 **约瑟夫·熊彼特（Joseph Schumpeter）** 提出的 **“创造性毁灭”驱动经济演进**的动态模型，强调**技术创新与企业家精神是经济周期的根本引擎**。其重要理论基础是：**创造性毁灭”（Creative Destruction），****定义**：新技术/新业态淘汰旧产业（如数码相机毁灭胶卷行业）→ **资源从低效领域向高效领域转移：**企业家创新 → 垄断利润 → 模仿者涌入 → 竞争加剧 → 利润消失 → 经济衰退 → 等待下一轮创新。\n\n**达里奥模型（Dalio Model）** 是桥水基金创始人 **瑞·达里奥（Ray Dalio）** 提出的 **债务周期理论**，核心揭示 **信贷扩张与收缩如何驱动经济系统性兴衰**。其模型融合了短债务周期（5-8年）与长债务周期（50-75年）。\n"
	// autocorrect: true

	content, err := os.ReadFile(path.Join("test", "test_bold.json"))
	assert.Nil(err)

	var topic struct {
		Talk struct {
			Text string `json:"text"`
		} `json:"talk"`
	}
	err = json.Unmarshal(content, &topic)
	assert.Nil(err)

	var output = topic.Talk.Text
	for _, f := range getFormatFuncs() {
		output, err = f(output)
		assert.Nil(err)
	}
	assert.Equal(expect, output)
}

func TestFormatTags(t *testing.T) {
	testText := []testCase{
		{`今天的话题是<e type="hashtag" hid="1234567890" title="%23%E4%BB%8A%E6%97%A5%E8%AF%9D%E9%A2%98%23" />，我们还会讨论<e type="hashtag" hid="2345678901" title="%23%E6%8A%80%E6%9C%AF%E5%88%9B%E6%96%B0%23" />`, `今天的话题是\#今日话题，我们还会讨论\#技术创新`},
		{`什么是专家主义\n\n<e type="hashtag" hid="28511114181521" title="%23%E6%88%90%E9%95%BF%E5%B0%8F%E8%B0%88%23" />`, `什么是专家主义\n\n\#成长小谈`},
		{`<e type="hashtag" hid="48881848115558" title="%23%E8%AE%A4%E7%9F%A5%E4%B8%8E%E5%91%BD%E8%BF%90%23" /> <e type="hashtag" hid="28511114181521" title="%23%E6%88%90%E9%95%BF%E5%B0%8F%E8%B0%88%23" /> `, `\#认知与命运 \#成长小谈 `},
	}

	assert := assert.New(t)
	for _, tc := range testText {
		result, err := replaceHashTags(tc.input)
		assert.Nil(err)
		assert.Equal(tc.expect, result)
	}
}

func TestBookMarkup(t *testing.T) {
	// autocorrect: false
	testCases := []testCase{
		{`<e type="web" href="https%3A%2F%2Fwx.zsxq.com%2Fmweb%2Fviews%2Fweread%2Fsearch.html%3Fkeyword%3D%E4%BA%BA%E6%80%A7%E7%9A%84%E5%BC%B1%E7%82%B9" title="人性的弱点" style="book" />`, `人性的弱点`},
		{`多次推荐《<e type="web" href="https%3A%2F%2Fwx.zsxq.com%2Fmweb%2Fviews%2Fweread%2Fsearch.html%3Fkeyword%3D%E4%BA%BA%E6%80%A7%E7%9A%84%E5%BC%B1%E7%82%B9" title="人性的弱点" style="book" />》这本书。`, `多次推荐《人性的弱点》这本书。`},
		{`<e type="web" href="https%3a%2f%2fwx.zsxq.com%2fmweb%2fviews%2fweread%2fsearch.html%3fkeyword%3d%e8%87%aa%e6%81%8b%e5%88%91%e8%ad%a6" title="%e8%87%aa%e6%81%8b%e5%88%91%e8%ad%a6" style="book" />`, `自恋刑警`},
		{`《<e type="web" href="https%3A%2F%2Fwx.zsxq.com%2Fmweb%2Fviews%2Fweread%2Fsearch.html%3Fkeyword%3D%E8%87%AA%E6%81%8B%E5%88%91%E8%AD%A6" title="%E8%87%AA%E6%81%8B%E5%88%91%E8%AD%A6" style="book" />》`, `《自恋刑警》`},
	}
	// autocorrect: true

	assert := assert.New(t)
	for _, tc := range testCases {
		output, err := replaceBookMarkUp(tc.input)
		assert.Nil(err)
		assert.Equal(tc.expect, output)
	}
}

func TestDecode(t *testing.T) {
	var testText = []testCase{
		{`%E6%AF%9B%E9%80%89`, `毛选`},
		{`如果你想要知道当年伟人是怎么做的，去读《%E6%AF%9B%E9%80%89》吧。`, `如果你想要知道当年伟人是怎么做的，去读《毛选》吧。`},
		{`图中的链接 [%E5%A4%B8%E5%AD%A9%E5%AD%90%E8%81%AA%E6%98%8E%E5%92%8C%E5%A4%B8%E5%AD%A9%E5%AD%90%E5%8A%AA%E5%8A%9B%E7%9C%9F%E7%9A%84%E4%B8%8D%E5%90%8C%E5%90%97%EF%BC%9F%20-%20%E7%9F%A5%E4%B9%8E](https%3A%2F%2Fwww.zhihu.com%2Fquestion%2F21724973)`, `图中的链接 [夸孩子聪明和夸孩子努力真的不同吗？ - 知乎](https://www.zhihu.com/question/21724973)`},
	}

	assert := assert.New(t)
	for _, tc := range testText {
		output, err := replacePercentEncodedChars(tc.input)
		assert.Nil(err)
		assert.Equal(tc.expect, output)
	}
}

func TestMention(t *testing.T) {
	testText := []testCase{
		{`<e type="mention" uid="585248841522544" title="%40Y.Z" />`, `@Y.Z`},
		{`111111<e type="mention" uid="585248841522544" title="%40Y.Z" />2222222`, `111111@Y.Z2222222`},
	}

	assert := assert.New(t)
	for _, tc := range testText {
		output, err := processMention(tc.input)
		assert.Nil(err)
		assert.Equal(tc.expect, output)
	}
}
