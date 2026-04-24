package main

import (
	"msm_backend/config"
	"msm_backend/controllers"
	"msm_backend/models"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	config.ConnectDB()
	config.DB.AutoMigrate(&models.Stock{}, &models.StockHistory{}, &models.Product{}, &models.Size{}, &models.Category{})
	r := gin.Default()
	r.Use(cors.Default())

	r.GET("/stocks", controllers.GetStock)
	r.GET("/products", controllers.GetProductsGrouped)
	r.GET("/stock/history", controllers.GetStockHistory)
	r.GET("/stock/low-alerts", controllers.GetLowStock)
	r.GET("/stock/export", controllers.ExecuteStockCSV)
	r.GET("/stock/history/export", controllers.ExportHistoryCSV)
	r.POST("/stock/add", controllers.AddStock)
	r.POST("/stock/reduce", controllers.ReduceStock)
	r.POST("/stock/threshold", controllers.UpdateStockThreshold)
	r.POST("/stock/bulk-restock", func(c *gin.Context) {
		controllers.BulkUpdateStock(c, "restock")
	})
	r.POST("/stock/bulk-sale", func(c *gin.Context) {
		controllers.BulkUpdateStock(c, "sale")
	})
	r.POST("/stock/bulk-return", func(c *gin.Context) {
		controllers.BulkUpdateStock(c, "return")
	})
	r.DELETE("stock/history/:id", controllers.DeleteStockHistory)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
	}

	r.Run(":" + port)
}
