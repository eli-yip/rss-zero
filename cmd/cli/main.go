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
	crawl     bool
	backtrack bool
	refmt     bool

	export    bool
	startTime string
	endTime   string

	zhihu   *zhihuOption
	zsxq    *zsxqOption
	xiaobot *xiaobotOption
}

type zhihuOption struct {
	userID  string
	answer  bool
	article bool
	pin     bool
	dC0     string
}

type zsxqOption struct {
	groupID int
	t       string
	digest  bool
	author  string
}

type xiaobotOption struct{ paperID string }

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
	logger.Info("parse args successfully", zap.Any("args", opt))

	if opt.zhihu != nil {
		if !opt.refmt {
			handleZhihu(opt, logger)
		}
		if opt.refmt {
			refmtZhihu(opt, logger)
		}
	}

	if opt.zsxq != nil {
		handleZsxq(opt, logger)
	}

	if opt.xiaobot != nil {
		if opt.export {
			exportXiaobot(opt, logger)
		}
	}
}

func parseArgs() (opt option, err error) {
	crawl := flag.Bool("crawl", false, "whether to crawl")

	userID := flag.String("user", "", "user id")
	answer := flag.Bool("answer", false, "answer")
	article := flag.Bool("article", false, "article")
	pin := flag.Bool("pin", false, "pin")
	backtrack := flag.Bool("backtrack", false, "whether to backtrack")
	dC0 := flag.String("d_c0", "", "d_c0 cookie")

	groupID := flag.Int("group", 0, "group id")
	authorID := flag.String("author", "", "author id")
	t := flag.String("type", "", "type")
	digest := flag.Bool("digest", false, "digest")

	paperID := flag.String("paper", "", "paper id")

	export := flag.Bool("export", false, "whether to export")
	startTime := flag.String("start", "", "start time")
	endTime := flag.String("end", "", "end time")

	refmt := flag.Bool("refmt", false, "whether to refmt")

	flag.Parse()

	setFlag := 0
	if *crawl {
		setFlag++
	}
	if *export {
		setFlag++
	}
	if *refmt {
		setFlag++
	}
	if setFlag != 1 {
		return option{}, errors.New("only support one of crawl, export and refmt")
	}

	if *userID == "" && *groupID == 0 && *paperID == "" {
		return option{}, errors.New("user id or group id is required")
	}

	setFlag = 0
	if *userID != "" {
		setFlag++
	}
	if *groupID != 0 {
		setFlag++
	}
	if *paperID != "" {
		setFlag++
	}
	if setFlag != 1 {
		return option{}, errors.New("only support one of user id, group id and paper id")
	}

	if *crawl {
		opt.crawl = true
		opt.backtrack = *backtrack

		// parse zhihu config
		if *userID != "" {
			opt.zhihu = &zhihuOption{}

			opt.zhihu.userID = *userID
			opt.zhihu.answer = *answer
			opt.zhihu.article = *article
			opt.zhihu.pin = *pin
			opt.zhihu.dC0 = *dC0

			return opt, nil
		}

		// parse zsxq config
		if *groupID != 0 {
			opt.zsxq = &zsxqOption{}

			opt.zsxq.groupID = *groupID
			return opt, nil
		}
	}

	if *export {
		opt.export = true
		opt.startTime = *startTime
		opt.endTime = *endTime

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

			opt.zhihu.userID = *userID
		}

		if *groupID != 0 {
			opt.zsxq = &zsxqOption{}
			opt.zsxq.groupID = *groupID
			opt.zsxq.author = *authorID
			opt.zsxq.t = *t
			opt.zsxq.digest = *digest
		}

		if *paperID != "" {
			opt.xiaobot = &xiaobotOption{}
			opt.export = true
			opt.xiaobot.paperID = *paperID
		}

		return opt, nil
	}

	if *refmt {
		opt.refmt = true

		if *userID != "" {
			opt.zhihu = &zhihuOption{}
			opt.zhihu.userID = *userID
		}

		if *groupID != 0 {
			opt.zsxq = &zsxqOption{}
			opt.zsxq.groupID = *groupID
		}

		return opt, nil
	}

	return option{}, errors.New("unknown error")
}

func parseExportTime(ts string) (t time.Time, err error) {
	if ts == "" {
		return time.Time{}, nil
	}

	t, err = time.Parse("2006-01-02", ts)
	if err != nil {
		return time.Time{}, err
	}

	return t.In(config.BJT), nil
}
