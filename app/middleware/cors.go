package middleware

import "github.com/gin-gonic/gin"

func V1Cors(c *gin.Context) {
	origin := c.Request.Header.Get("Origin")
	if origin == "" {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Add("Vary", "Origin")
	}
	c.Writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Token, Content-Type, Accept")
	c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	if c.Request.Method == "OPTIONS" {
		c.AbortWithStatus(204)
		return
	}
	c.Next()
}
