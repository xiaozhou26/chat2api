package handler

import (
	"chat2api/app/conf"
	"chat2api/app/router"
	"chat2api/pkg/logx"
	"context"
	"net/http"
	"sync"

	"github.com/sirupsen/logrus"
)

var initOnce sync.Once

func Handler(w http.ResponseWriter, r *http.Request) {
	initOnce.Do(func() {
		ctx := logx.TagContext(context.Background(), "vercel")
		logx.SetLevel(logrus.DebugLevel)
		logx.SetFormatter(&logx.TextFormatter{})
		conf.InitServerless(ctx)
	})
	router.HandlerGinEngine(w, r)
}
