package cron

import (
	"testing"
	"testing/synctest"
)

func TestGenerateRealCrawlFuncWaitsForCompletion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		done := make(chan struct{})
		crawl := GenerateRealCrawlFunc(func(info chan CronJobInfo) {
			info <- CronJobInfo{}
			<-release
		})
		go func() {
			crawl()
			close(done)
		}()

		synctest.Wait()
		select {
		case <-done:
			t.Error("scheduled task returned before crawl completed")
		default:
		}

		close(release)
		synctest.Wait()
		select {
		case <-done:
		default:
			t.Error("scheduled task did not return after crawl completed")
		}
	})
}

func TestGenerateRealCrawlFuncWithoutNotification(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		done := make(chan struct{})
		go func() {
			GenerateRealCrawlFunc(func(chan CronJobInfo) {})()
			close(done)
		}()

		synctest.Wait()
		select {
		case <-done:
		default:
			t.Error("scheduled task waited for a notification after crawl returned")
		}
	})
}
