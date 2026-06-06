package service

import (
	"chat2api/app/token_pool"
	"chat2api/pkg/logx"
	"fmt"

	"github.com/gin-gonic/gin"
)

type AccessTokensResp struct {
	Count       int `json:"count"`
	CanUseCount int `json:"can_use_count"`
}

func AccTokens(c *gin.Context) {
	resp := &AccessTokensResp{
		Count:       token_pool.GetAccessTokenPool().Size(),
		CanUseCount: token_pool.GetAccessTokenPool().CanUseSize(),
	}
	logx.WithContext(c.Request.Context()).Info(fmt.Sprint("AccessTokenPool Tokens: ", resp.Count))
	c.JSON(200, resp)
}
