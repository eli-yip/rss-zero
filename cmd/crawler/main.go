package main

import (
	"errors"
	"flag"
	"time"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/pkg/log"
	"go.uber.org/zap"
)

type option struct {
	crawl bool

	export    bool
	startTime string
	endTime   string

	zhihu *zhihuOption
	zsxq  *zsxqOption
}

type zhihuOption struct {
	userID     string
	answer     bool
	article    bool
	pin        bool
	answerURL  string
	articleURL string
	pinURL     string
	dC0        string
}

type zsxqOption struct{ groupID string }

func main() {
	logger := log.NewLogger()
	defer func() {
		if r := recover(); r != nil {
			logger.Fatal("panic", zap.Any("panic", r))
		} else {
			logger.Info("done")
		}
	}()

	var err error

	config.InitFromEnv()
	logger.Info("init config successfully")

	opt, err := parseArgs()
	if err != nil {
		logger.Fatal("fail to parse args", zap.Error(err))
	}

	if opt.zhihu != nil {
		handleZhihu(opt, logger)
	}
}

func parseArgs() (opt option, err error) {
	crawl := flag.Bool("crawl", false, "whether to crawl")

	userID := flag.String("user", "", "user id")
	answer := flag.Bool("answer", false, "answer")
	article := flag.Bool("article", false, "article")
	pin := flag.Bool("pin", false, "pin")
	answerURL := flag.String("answer_url", "", "answer url")
	articleURL := flag.String("article_url", "", "article url")
	pinURL := flag.String("pin_url", "", "pin url")
	dC0 := flag.String("d_c0", "", "d_c0 cookie")

	groupID := flag.String("group", "", "group id")

	export := flag.Bool("export", false, "whether to export")
	startTime := flag.String("start", "", "start time")
	endTime := flag.String("end", "", "end time")

	flag.Parse()

	if *export && *crawl {
		return option{}, errors.New("export type and parse type can't be set at the same time")
	}

	if *crawl {
		if *userID != "" && *groupID != "" {
			return option{}, errors.New("user id and group id can't be set at the same time")
		}
		if *userID == "" && *groupID == "" {
			return option{}, errors.New("user id or group id is required")
		}

		if *userID != "" {
			opt.zhihu = &zhihuOption{}

			opt.crawl = true
			opt.zhihu.userID = *userID
			opt.zhihu.answer = *answer
			opt.zhihu.article = *article
			opt.zhihu.pin = *pin
			opt.zhihu.answerURL = *answerURL
			opt.zhihu.articleURL = *articleURL
			opt.zhihu.pinURL = *pinURL
			opt.zhihu.dC0 = *dC0

			return opt, nil
		}

		if *groupID != "" {
			opt.zsxq = &zsxqOption{}

			opt.crawl = true
			opt.zsxq.groupID = *groupID
			return opt, nil
		}
	}

	if *export {
		if *userID != "" {
			opt.zhihu = &zhihuOption{}

			setFlag := 0
			if *answer {
				opt.zhihu.answer = true
				setFlag++
			}
			if *article {
				opt.zhihu.article = true
				setFlag++
			}
			if *pin {
				opt.zhihu.pin = true
				setFlag++
			}

			if setFlag != 1 {
				return option{}, errors.New("export type can only be set once")
			}

			opt.export = true
			opt.zhihu.userID = *userID
		} else if *groupID != "" {
			opt.zsxq = &zsxqOption{}
			opt.zsxq.groupID = *groupID
		} else {
			return option{}, errors.New("user id or group id is required")
		}

		opt.export = true
		opt.startTime = *startTime
		opt.endTime = *endTime
		return opt, nil
	}

	return option{}, errors.New("crawl or export is required")
}

func parseExportTime(ts string) (t time.Time, err error) {
	location, _ := time.LoadLocation("Asia/Shanghai")
	if ts == "" {
		return time.Time{}, nil
	}
	t, err = time.Parse("2006-01-02", ts)
	if err != nil {
		return time.Time{}, err
	}
	return t.In(location), nil
}
