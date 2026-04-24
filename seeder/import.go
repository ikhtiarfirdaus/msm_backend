package seeder

import (
	"encoding/csv"
	"fmt"
	"msm_backend/config"
	"msm_backend/models"
	"os"
	"strconv"
	"strings"
)

func ImportAll() {
	file, err := os.Open("data.csv")
	if err != nil {
		fmt.Println("❌ Gagal buka data.csv:", err)
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, _ := reader.ReadAll()

	fmt.Println("--- Memulai Import Produk & Stok ---")

	for i, row := range records {
		// Skip header dan pastikan kolom cukup (ID, Produk, Kat, Harga, Qty)
		if i == 0 || len(row) < 5 {
			continue
		}

		code := strings.TrimSpace(row[0])
		nameFull := strings.TrimSpace(row[1])
		categoryName := strings.TrimSpace(row[2])
		price, _ := strconv.Atoi(strings.TrimSpace(row[3]))
		qty, _ := strconv.Atoi(strings.TrimSpace(row[4]))

		// Split Nama dan Size
		parts := strings.Split(nameFull, "-")
		productName := nameFull
		sizeName := "All Size"
		if len(parts) >= 2 {
			productName = strings.TrimSpace(parts[0])
			sizeName = strings.TrimSpace(parts[1])
		}

		// 1. Sinkronisasi Category
		var category models.Category
		config.DB.FirstOrCreate(&category, models.Category{Name: categoryName})

		// 2. Sinkronisasi Product
		var product models.Product
		config.DB.Where(models.Product{Name: productName}).Attrs(models.Product{
			Price:      price,
			CategoryID: category.ID,
		}).FirstOrCreate(&product)

		// 3. Sinkronisasi Size
		var size models.Size
		config.DB.FirstOrCreate(&size, models.Size{Name: sizeName})

		// 4. Sinkronisasi Stock (Kunci Utama: Code)
		var stock models.Stock
		result := config.DB.Where(models.Stock{Code: code}).First(&stock)

		if result.RowsAffected == 0 {
			// Jika belum ada, buat baru
			config.DB.Create(&models.Stock{
				ProductID: product.ID,
				SizeID:    size.ID,
				Code:      code,
				Qty:       qty,
			})
		} else {
			// Jika sudah ada, update Qty, ProductID, dan SizeID agar sinkron
			config.DB.Model(&stock).Updates(models.Stock{
				ProductID: product.ID,
				SizeID:    size.ID,
				Qty:       qty,
			})
		}
	}
	fmt.Println("✅ Berhasil! Semua produk dan stok telah sinkron.")
}
