package cron

import "time"

const defaultFetchCount = 20

const rssTTL = time.Hour * 2

var longLongago = time.Date(2008, 1, 1, 0, 0, 0, 0, time.UTC)
