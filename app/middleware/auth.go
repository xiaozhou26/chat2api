package middleware

import (
	"chat2api/app/common"
	"chat2api/app/conf"
	"strings"

	"github.com/gin-gonic/gin"
)

func V1Auth(c *gin.Context) {
	authToken := c.Request.Header.Get("Authorization")
	localToken := strings.TrimSpace(strings.TrimPrefix(authToken, "Bearer "))
	appConf := conf.GetApp()
	if strings.HasPrefix(localToken, "at-") {
		c.Next()
		return
	}
	if strings.HasPrefix(authToken, "Bearer eyJhbGciOiJSUzI1NiI") {
		c.Next()
		return
	}
	if len(appConf.Auth.AccessTokens) == 0 {
		common.ErrorResponse(c, 401, "No local API keys are configured", nil)
		return
	}
	if authToken == "" {
		common.ErrorResponse(c, 401, "You didn't provide an API key. You need to provide your API key in an Authorization header using Bearer auth (i.e. Authorization: Bearer YOUR_KEY)", nil)
		return
	}
	if !common.IsStrInArray(localToken, appConf.Auth.AccessTokens) {
		common.ErrorResponse(c, 401, "Incorrect API key provided: sk-4yNZz***************************************6mjw.", nil)
		return
	}
	c.Next()
}
