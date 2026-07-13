package job

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/internal/ai"
	notify "github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	"github.com/eli-yip/rss-zero/pkg/cron"
	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
	githubCron "github.com/eli-yip/rss-zero/pkg/routers/github/cron"
	xiaobotCron "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/cron"
	zhihuCron "github.com/eli-yip/rss-zero/pkg/routers/zhihu/cron"
	zsxqCron "github.com/eli-yip/rss-zero/pkg/routers/zsxq/cron"
)

// BuildDeps bundles every dependency a source's Build closure may need. Only the
// deps the four sources actually use live here (no fileService: no source needs it).
type BuildDeps struct {
	Redis    redis.Redis
	Cookie   cookie.CookieIface
	DB       *gorm.DB
	AI       ai.AI
	Notifier notify.Notifier
}

// ResumeInfo is the cross-source resume input. Sources that can't resume ignore it;
// resumable sources map it onto their own package's ResumeJobInfo.
type ResumeInfo struct{ JobID, LastCrawled string }

// SourceSpec 是动态 cron 来源的唯一注册点：Kind 同时作为 API 字符串、注册表键和数据库值。
type SourceSpec struct {
	Kind      string // zsxq/zhihu/xiaobot/github
	Resumable bool   // 仅 zsxq/zhihu 支持续跑
	Build     func(deps BuildDeps, def *cronDB.CronTask, resume *ResumeInfo) CrawlFunc
}

// JobName is the scheduler job name for this source, e.g. "zsxq_crawl".
func (s SourceSpec) JobName() string { return s.Kind + "_crawl" }

var registry = []SourceSpec{
	{Kind: "zsxq", Resumable: true, Build: buildZsxq},
	{Kind: "zhihu", Resumable: true, Build: buildZhihu},
	{Kind: "xiaobot", Resumable: false, Build: buildXiaobot},
	{Kind: "github", Resumable: false, Build: buildGitHub},
}

func buildZsxq(deps BuildDeps, def *cronDB.CronTask, resume *ResumeInfo) CrawlFunc {
	var r *zsxqCron.ResumeJobInfo
	if resume != nil {
		r = &zsxqCron.ResumeJobInfo{JobID: resume.JobID, LastCrawled: resume.LastCrawled}
	}
	return zsxqCron.BuildCrawlFunc(r, def.ID, def.Include, def.Exclude, deps.Redis, deps.Cookie, deps.DB, deps.AI, deps.Notifier)
}

func buildZhihu(deps BuildDeps, def *cronDB.CronTask, resume *ResumeInfo) CrawlFunc {
	var r *zhihuCron.ResumeJobInfo
	if resume != nil {
		r = &zhihuCron.ResumeJobInfo{JobID: resume.JobID, LastCrawled: resume.LastCrawled}
	}
	return zhihuCron.BuildCrawlFunc(r, def.ID, def.Include, def.Exclude, deps.Redis, deps.Cookie, deps.DB, deps.AI, deps.Notifier)
}

func buildXiaobot(deps BuildDeps, def *cronDB.CronTask, _ *ResumeInfo) CrawlFunc {
	return xiaobotCron.BuildCronCrawlFunc(deps.Redis, deps.Cookie, deps.DB, deps.Notifier, &xiaobotCron.Filter{
		Include: def.Include,
		Exclude: def.Exclude,
	})
}

func buildGitHub(deps BuildDeps, _ *cronDB.CronTask, _ *ResumeInfo) CrawlFunc {
	return githubCron.Crawl(deps.Redis, deps.Cookie, deps.DB, deps.AI, deps.Notifier)
}

// AddToScheduler 构建抓取函数、注册调度任务，并记录任务定义与调度器任务的进程内映射。
func AddToScheduler(cronService *cron.CronService, jobIndex *JobIndex, spec SourceSpec, deps BuildDeps, def *cronDB.CronTask) (jobID string, err error) {
	fn := spec.Build(deps, def, nil)
	if jobID, err = cronService.AddCrawlJob(spec.JobName(), def.CronExpr, fn); err != nil {
		return "", fmt.Errorf("failed to add crawl job: %w", err)
	}
	jobIndex.Set(def.ID, jobID)
	return jobID, nil
}

// SpecByKind 按统一的来源 Kind 查找注册项。
func SpecByKind(kind string) (SourceSpec, bool) {
	for _, s := range registry {
		if s.Kind == kind {
			return s, true
		}
	}
	return SourceSpec{}, false
}
