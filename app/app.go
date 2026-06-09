package app

import (
	"chat2api/app/conf"
	"chat2api/app/env"
	"chat2api/app/router"
	"chat2api/pkg/logx"
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

func Run(ctx context.Context) {

	cleanups := &stack{}
	{
		logx.SetLevel(logrus.DebugLevel)
		logx.SetFormatter(func() logrus.Formatter {
			return &logx.TextFormatter{}
		}())
		ctx = logx.TagContext(ctx, "initial")
		logx.WithContext(ctx).Infof("application process in PID: %v", os.Getpid())
	}

	// conf
	cleanups.push(conf.Init(ctx))

	// gin
	cleanups.push(router.Init(ctx))

	switch env.Curr {
	case env.DEV:
	case env.TEST:
	case env.PROD:
	}

	code := 1
	// 信号通道使用缓冲区，避免在高并发日志/清理阶段因接收滞后丢失退出信号。
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer signal.Stop(s)
EXIT:
	for {
		switch <-s {
		case syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT:
			code = 0
			break EXIT
		case syscall.SIGHUP:
		default:
			break EXIT
		}
	}

	cleanupCtx := logx.TagContext(ctx, "cleanup")
	for cleanups.next() {
		cleanups.pop()(cleanupCtx)
	}
	time.Sleep(time.Second)
	os.Exit(code)
}

type stack struct {
	sync.Mutex
	items []func(context.Context)
}

func (s *stack) pop() func(context.Context) {
	s.Lock()
	defer s.Unlock()
	item := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return item
}

func (s *stack) push(item func(context.Context)) {
	s.Lock()
	defer s.Unlock()
	s.items = append(s.items, item)
}

func (s *stack) next() bool {
	return len(s.items) > 0
}
