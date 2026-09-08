package response

import "github.com/gin-gonic/gin"

// Meta contiene metadatos de paginación.
type Meta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
}

// Success envía una respuesta exitosa estandarizada.
func Success(c *gin.Context, status int, data interface{}) {
	c.JSON(status, gin.H{
		"success": true,
		"data":    data,
	})
}

// SuccessWithMeta envía una respuesta exitosa paginada.
func SuccessWithMeta(c *gin.Context, status int, data interface{}, meta Meta) {
	c.JSON(status, gin.H{
		"success": true,
		"data":    data,
		"meta":    meta,
	})
}

// Error envía una respuesta de error estandarizada.
func Error(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{
		"success": false,
		"error":   message,
	})
}
