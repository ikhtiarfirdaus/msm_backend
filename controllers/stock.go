package controllers

import (
	"encoding/csv"
	"fmt"
	"msm_backend/config"
	"msm_backend/models"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// =========================
// REQUEST STRUCT (AMAN)
// =========================
type StockRequest struct {
	ProductID uint `json:"product_id"`
	SizeID    uint `json:"size_id"`
	Qty       int  `json:"qty"`
}

// logic server
func ExecuteStockUpdate(tx *gorm.DB, code string, amount int, mode string) error {
	var stock models.Stock
	// 1. Cari produk berdasarkan barcode/code
	if err := tx.Where("code = ?", code).First(&stock).Error; err != nil {
		return fmt.Errorf("produk dengan kode %s tidak ditemukan", code)
	}

	// 2. Tentukan arah perubahan Qty
	qtyChange := 0
	switch mode {
	case "restock":
		qtyChange = amount // Barang masuk (positif)
	case "sale", "return":
		qtyChange = -amount // Barang keluar/berkurang (negatif)
	default:
		return fmt.Errorf("mode '%s' tidak valid", mode)
	}

	// 3. Hitung stok akhir
	newQty := stock.Qty + qtyChange

	// 4. Update tabel stok utama
	if err := tx.Model(&stock).Update("qty", newQty).Error; err != nil {
		return err
	}

	// 5. Catat riwayat (History) dengan label spesifik
	history := models.StockHistory{
		StockID:   stock.ID,
		QtyChange: qtyChange,
		QtyFinal:  newQty,
		Type:      mode, // Menyimpan "restock", "sale", atau "return"
	}

	return tx.Create(&history).Error
}

func BulkUpdateStock(c *gin.Context, mode string) {
	// 1. Ambil file dari form-data
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file csv wajib di upload"})
		return
	}

	// 2. Simpan sementara & buka file
	tempPath := "temp_" + file.Filename
	c.SaveUploadedFile(file, tempPath)
	defer os.Remove(tempPath)

	f, _ := os.Open(tempPath)
	defer f.Close()

	// 3. Baca isi CSV
	reader := csv.NewReader(f)
	records, _ := reader.ReadAll()

	// 4. Jalankan transaksi database
	tx := config.DB.Begin()

	for i, row := range records {
		if i == 0 || len(row) < 2 { // Skip header
			continue
		}

		code := strings.TrimSpace(row[0])
		amount, _ := strconv.Atoi(strings.TrimSpace(row[1]))

		// Panggil logic pusat dengan mode yang dipilih
		if err := ExecuteStockUpdate(tx, code, amount, mode); err != nil {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Gagal di baris %d (%s): %v", i+1, code, err),
			})
			return
		}
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Berhasil memproses bulk %s", mode)})
}

func GetStockHistory(c *gin.Context) {
	var histories []models.StockHistory

	err := config.DB.
		Preload("Stock.Product").Preload("Stock.Size").Order("created_at desc").
		Limit(100).Find(&histories).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "gagal mengambil riwayat stock",
		})
		return
	}

	var result []gin.H
	for _, h := range histories {
		result = append(result, gin.H{
			"id":           h.ID,
			"product_name": h.Stock.Product.Name,
			"size":         h.Stock.Size.Name,
			"code":         h.Stock.Code,
			"qty_change":   h.QtyChange,
			"qty_final":    h.QtyFinal,
			"type":         h.Type,
			"created_at":   h.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	c.JSON(http.StatusOK, result)
}

func GetLowStock(c *gin.Context) {
	var lowStocks []models.Stock

	err := config.DB.Preload("Product").
		Preload("Size").Where("qty <= low_stock_threshold").Find(&lowStocks).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengambil stock tipis"})
		return
	}
	c.JSON(http.StatusOK, lowStocks)
}

// =========================
// GET ALL STOCK
// =========================
func GetStock(c *gin.Context) {
	var stocks []models.Stock

	config.DB.
		Preload("Product").
		Preload("Size").
		Find(&stocks)

	resultMap := make(map[string]gin.H)

	for _, s := range stocks {

		key := s.Product.Name

		if _, ok := resultMap[key]; !ok {
			resultMap[key] = gin.H{
				"name":     s.Product.Name,
				"price":    s.Product.Price,
				"category": s.Product.CategoryID,
				"sizes":    []gin.H{},
			}
		}

		resultMap[key]["sizes"] = append(
			resultMap[key]["sizes"].([]gin.H),
			gin.H{
				"code":  s.Code,
				"size":  s.Size.Name,
				"stock": s.Qty,
			},
		)
	}

	var result []gin.H
	for _, v := range resultMap {
		result = append(result, v)
	}

	c.JSON(http.StatusOK, result)
}

func UpdateStockThreshold(c *gin.Context) {
	var input struct {
		Code      string `json:"code" binding:"required"`
		Threshold int    `json:"threshold" binding:"required,min=0"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "format data salah"})
		return
	}
	result := config.DB.Model(&models.Stock{}).
		Where("code = ?", input.Code).Update("low_stock_threshold", input.Threshold)

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "produk dengan kode tersebut tidak di temukan"})
		return
	}
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal update database"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":       "threshold berhasil di perbarui",
		"code":          input.Code,
		"new_threshold": input.Threshold,
	})

}

// =========================
// ADD STOCK
// =========================
func AddStock(c *gin.Context) {
	var input StockRequest

	// bind JSON
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Format JSON salah",
		})
		return
	}

	// validasi
	if input.ProductID == 0 || input.SizeID == 0 || input.Qty <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "product_id, size_id, qty wajib diisi dengan benar",
		})
		return
	}

	// cek apakah product ada
	var product models.Product
	if err := config.DB.First(&product, input.ProductID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Product tidak ditemukan",
		})
		return
	}

	// cek apakah size ada
	var size models.Size
	if err := config.DB.First(&size, input.SizeID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Size tidak ditemukan",
		})
		return
	}

	// cek stock existing
	var stock models.Stock
	result := config.DB.
		Where("product_id = ? AND size_id = ?", input.ProductID, input.SizeID).
		First(&stock)

	if result.RowsAffected == 0 {
		// CREATE BARU (pakai model, bukan request!)
		newStock := models.Stock{
			ProductID: input.ProductID,
			SizeID:    input.SizeID,
			Qty:       input.Qty,
		}

		if err := config.DB.Create(&newStock).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, newStock)
		return
	}

	// UPDATE STOCK
	stock.Qty += input.Qty

	if err := config.DB.Save(&stock).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, stock)
}

// =========================
// REDUCE STOCK
// =========================
func ReduceStock(c *gin.Context) {
	var input StockRequest

	// bind JSON
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Format JSON salah",
		})
		return
	}

	// validasi
	if input.ProductID == 0 || input.SizeID == 0 || input.Qty <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "product_id, size_id, qty wajib diisi dengan benar",
		})
		return
	}

	// ambil stock
	var stock models.Stock
	err := config.DB.
		Where("product_id = ? AND size_id = ?", input.ProductID, input.SizeID).
		First(&stock).Error

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Stock tidak ditemukan",
		})
		return
	}

	// cek cukup atau tidak
	if stock.Qty < input.Qty {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Stock tidak cukup",
		})
		return
	}

	// kurangi
	stock.Qty -= input.Qty

	if err := config.DB.Save(&stock).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, stock)
}

func GetProductsClean(c *gin.Context) {
	var products []models.Product

	config.DB.Preload("Category").Find(&products)

	var result []gin.H

	for _, p := range products {

		var stocks []models.Stock
		config.DB.
			Preload("Size").
			Where("product_id = ?", p.ID).
			Find(&stocks)

		var sizes []gin.H

		for _, s := range stocks {
			sizes = append(sizes, gin.H{
				"size":  s.Size.Name,
				"stock": s.Qty,
				"code":  s.Code, // 🔥 FIX disini
			})
		}

		result = append(result, gin.H{
			"name":     p.Name,
			"category": p.Category.Name,
			"price":    p.Price,
			"sizes":    sizes,
		})
	}

	c.JSON(200, result)
}

func GetProductsGrouped(c *gin.Context) {
	var stocks []models.Stock

	config.DB.
		Preload("Product.Category").
		Preload("Size").
		Find(&stocks)

	grouped := make(map[string]gin.H)

	for _, s := range stocks {

		key := s.Product.Name

		// kalau belum ada product
		if _, exists := grouped[key]; !exists {
			grouped[key] = gin.H{
				"name":     s.Product.Name,
				"category": s.Product.Category.Name,
				"price":    s.Product.Price,
				"sizes":    []gin.H{},
			}
		}

		item := grouped[key]
		sizes := item["sizes"].([]gin.H)

		// 🔥 CEK DUPLICATE SIZE
		found := false
		for i, sz := range sizes {
			if sz["size"] == s.Size.Name {
				// kalau size sama → update stock
				sizes[i]["stock"] = s.Qty
				found = true
				break
			}
		}

		// kalau belum ada → tambah
		if !found {
			sizes = append(sizes, gin.H{
				"size":  s.Size.Name,
				"stock": s.Qty,
				"code":  s.Code, // 🔥 FIX disini
			})
		}

		item["sizes"] = sizes
		grouped[key] = item
	}

	// convert ke array
	var result []gin.H
	for _, v := range grouped {
		result = append(result, v)
	}

	c.JSON(200, result)
}

func ExecuteStockCSV(c *gin.Context) {
	var stocks []models.Stock

	if err := config.DB.Preload("Product").Preload("Size").Find(&stocks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengambil data stock"})
		return
	}

	fileName := fmt.Sprintf("laporan_stok_%s.csv", time.Now().Format("2006-01-02"))
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	c.Header("Content-Type", "text/csv")

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	header := []string{"Kode Barcode", "Nama Produk", "Ukuran", "Stock Saat Ini", "Bata Aman (Threshold)"}
	if err := writer.Write(header); err != nil {
		return
	}

	for _, s := range stocks {
		row := []string{
			s.Code,
			s.Product.Name,
			s.Size.Name,
			strconv.Itoa(s.Qty),
			strconv.Itoa(s.LowStockThreshold),
		}
		if err := writer.Write(row); err != nil {
			fmt.Println("error writing", err)
		}
	}
	writer.Flush()
}

func ExportHistoryCSV(c *gin.Context) {
	var histories []models.StockHistory

	filterType := c.Query("type")

	query := config.DB.Preload("Stock.Product").Preload("Stock.Size").Order("created_at desc")

	if filterType != "" {
		query = query.Where("type = ?", strings.ToLower(filterType))
	}

	if err := query.Find(&histories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menarik data riwayat"})
		return
	}

	label := "semua"
	if filterType != "" {
		label = filterType
	}
	fileName := fmt.Sprintf("mutasi_%s_%s.csv", label, time.Now().Format("2006-01"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	c.Header("Content-Type", "text/csv")
	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	writer.Write([]string{"Tanggal", "Barcode", "Produk", "Ukuran", "Tipe", "Qty Change", "Qty Final"})

	for _, h := range histories {
		writer.Write([]string{
			h.CreatedAt.Format("2006-01-02 15:04:05"),
			h.Stock.Code,
			h.Stock.Product.Name,
			h.Stock.Size.Name,
			strings.ToUpper(h.Type),
			strconv.Itoa(h.QtyChange),
			strconv.Itoa(h.QtyFinal),
		})
	}
}

func DeleteStockHistory(c *gin.Context) {
	id := c.Param("id")
	var history models.StockHistory

	// 1. Mulai Transaksi Database
	tx := config.DB.Begin()

	// 2. Ambil data history lengkap dengan relasi Stock-nya
	// Ini krusial agar kita tahu berapa QtyChange yang harus dibatalkan
	if err := tx.Preload("Stock").First(&history, id).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"error": "Data history tidak ditemukan"})
		return
	}

	// 3. REVERSE LOGIC: Hitung saldo stok yang seharusnya
	// Rumus: Stok Sekarang - Perubahan Dulu
	// Contoh: Dulu sale -2, maka sekarang dikurangi (-2) alias ditambah 2.
	newQty := history.Stock.Qty - history.QtyChange

	// Safety Check: Mencegah stok jadi negatif jika restock besar dihapus
	if newQty < 0 {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Gagal: Menghapus restock ini akan membuat stok %s menjadi negatif (%d)", history.Stock.Code, newQty),
		})
		return
	}

	// 4. Update saldo di tabel Stocks
	if err := tx.Model(&models.Stock{}).Where("id = ?", history.StockID).Update("qty", newQty).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengembalikan saldo stok"})
		return
	}

	// 5. HARD DELETE: Hapus permanen dari tabel stock_histories
	// Menggunakan Unscoped() agar tidak terjebak di Soft Delete GORM
	if err := tx.Unscoped().Delete(&history).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus baris history"})
		return
	}

	// 6. Selesaikan transaksi
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit database"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "Transaksi berhasil dibatalkan (Void)",
		"history_id":      id,
		"reverted_qty_to": newQty,
	})
}

func SearchProduct(c *gin.Context) {
	query := c.Query("q")

	var products []models.Product

	config.DB.Preload("Category").
		Where("name LIKE ? OR code LIKE ?", "%"+query+"%", "%"+query+"%").
		Find(&products)

	c.JSON(200, products)
}

func TotalProduct(c *gin.Context) {
	var total int64

	config.DB.Model(&models.Product{}).Count(&total)

	c.JSON(200, gin.H{"total_product": total})
}

func TotalStock(c *gin.Context) {
	var total int64
	config.DB.Model(&models.Stock{}).
		Select("SUM(qty)").Scan(&total)

	c.JSON(200, gin.H{"total_stock": total})
}
