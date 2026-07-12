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

// SourceSpec is the single source of truth for one dynamic cron source: its DB type
// key, its API name, whether it can resume, and how to build its crawlFunc. Adding a
// source is one row here plus its Build closure — nothing else in this package branches
// on source type.
type SourceSpec struct {
	Type      int    // existing cronDB.TypeXxx; registry key (unchanged by this issue)
	Name      string // API string, e.g. "zsxq"
	Resumable bool   // only zsxq/zhihu can resume
	Build     func(deps BuildDeps, def *cronDB.CronTask, resume *ResumeInfo) CrawlFunc
}

// JobName is the scheduler job name for this source, e.g. "zsxq_crawl".
func (s SourceSpec) JobName() string { return s.Name + "_crawl" }

var registry = []SourceSpec{
	{Type: cronDB.TypeZsxq, Name: "zsxq", Resumable: true, Build: buildZsxq},
	{Type: cronDB.TypeZhihu, Name: "zhihu", Resumable: true, Build: buildZhihu},
	{Type: cronDB.TypeXiaobot, Name: "xiaobot", Resumable: false, Build: buildXiaobot},
	{Type: cronDB.TypeGitHub, Name: "github", Resumable: false, Build: buildGitHub},
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

// AddToScheduler builds a source's crawlFunc, registers it with the scheduler under
// the source's job name, and writes the resulting scheduler job id back onto the
// definition. Shared by the startup loader (addJobToCronService) and the request path
// (AddTask/PatchTask), which used to carry near-identical copies of this sequence.
func AddToScheduler(cronService *cron.CronService, cronDBService cronDB.DB, spec SourceSpec, deps BuildDeps, def *cronDB.CronTask) (jobID string, err error) {
	fn := spec.Build(deps, def, nil)
	if jobID, err = cronService.AddCrawlJob(spec.JobName(), def.CronExpr, fn); err != nil {
		return "", fmt.Errorf("failed to add crawl job: %w", err)
	}
	if err = cronDBService.PatchDefinition(def.ID, nil, nil, nil, &jobID); err != nil {
		return "", fmt.Errorf("failed to patch definition of job id: %w", err)
	}
	return jobID, nil
}

// SpecByType looks up a source by its DB type int.
func SpecByType(taskType int) (SourceSpec, bool) {
	for _, s := range registry {
		if s.Type == taskType {
			return s, true
		}
	}
	return SourceSpec{}, false
}

// specByName looks up a source by its API name. Unexported: no cross-package caller
// (only TypeStrToInt and tests use it); export it if Issue 3 needs name-keyed lookup.
func specByName(name string) (SourceSpec, bool) {
	for _, s := range registry {
		if s.Name == name {
			return s, true
		}
	}
	return SourceSpec{}, false
}

// TypeStrToInt converts an API source name to its DB type int.
func TypeStrToInt(name string) (int, error) {
	spec, ok := specByName(name)
	if !ok {
		return 0, fmt.Errorf("unknown task type: %s", name)
	}
	return spec.Type, nil
}

// TypeIntToStr converts a DB type int back to its API source name.
func TypeIntToStr(taskType int) (string, error) {
	spec, ok := SpecByType(taskType)
	if !ok {
		return "", fmt.Errorf("unknown task type: %d", taskType)
	}
	return spec.Name, nil
}
