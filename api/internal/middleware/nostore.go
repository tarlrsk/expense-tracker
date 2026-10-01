package middleware

import "github.com/gin-gonic/gin"

// NoStore marks every response, errors and 404s included, as not cacheable.
func NoStore() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "private, no-store")
		c.Next()
	}
}
