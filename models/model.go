package models

import "time"

type Category struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"unique"`
}

type Product struct {
	ID         uint   `gorm:"primaryKey"`
	Name       string `gorm:"unique"`
	Price      int
	CategoryID uint
	Category   Category
}

type Size struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"unique"`
}

type Stock struct {
	ID        uint    `gorm:"primaryKey"`
	ProductID uint    `gorm:"product_id"`
	Product   Product `gorm:"foreignKey:ProductID" json:"product"`
	SizeID    uint    `json:"size_id"`
	Size      Size    `gorm:"foreignKey:SizeID" json:"size"`
	Code      string  `gorm:"type:varchar(100);uniqueIndex"`
	Qty       int     `json:"qty"`

	LowStockThreshold int `gorm:"default:3" json:"low_stock_threshold"`

	// 🔥 ANTI DUPLICATE
	_ struct{} `gorm:"uniqueIndex:idx_product_size"`
}

type StockHistory struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	StockID     uint      `json:"stock_id"`
	Stock       Stock     `gorm:"foreignKey:StockID" json:"-"`
	QtyChange   int       `json:"qty_change"`
	QtyFinal    int       `json:"qty_final"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}
