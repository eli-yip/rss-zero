package common

import "go.uber.org/zap"

type Task struct {
	TextCh chan string
	ErrCh  chan error
	Logger *zap.Logger
}
