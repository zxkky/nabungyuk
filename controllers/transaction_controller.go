package controllers

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/nabungyuk/nabungyuk/config"
	"github.com/nabungyuk/nabungyuk/middleware"
	"github.com/nabungyuk/nabungyuk/models"
)

// IncomeCategories are the allowed income categories
var IncomeCategories = []string{"Gaji", "Uang Saku", "Freelance", "Bonus", "Bisnis", "Lainnya"}

// ExpenseCategories are the allowed expense categories
var ExpenseCategories = []string{"Makanan", "Transportasi", "Belanja", "Hiburan", "Pendidikan", "Tagihan", "Kesehatan", "Keluarga", "Subscription", "Lainnya"}

// MaxReceiptSize is the maximum allowed upload size in bytes (5MB)
const MaxReceiptSize = 5 * 1024 * 1024

// allowedImageTypes maps file extensions to allowed MIME types
var allowedImageTypes = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
}

// TransactionController handles transaction-related requests
type TransactionController struct{}

// NewTransactionController creates a new TransactionController
func NewTransactionController() *TransactionController {
	return &TransactionController{}
}

// transactionResponse is the JSON response shape for a transaction
type transactionResponse struct {
	ID              uint      `json:"id"`
	Type            string    `json:"type"`
	Title           string    `json:"title"`
	Category        string    `json:"category"`
	Amount          int64     `json:"amount"`
	Receipt         string    `json:"receipt"`
	ReceiptURL      string    `json:"receipt_url"`
	Note            string    `json:"note"`
	TransactionDate time.Time `json:"transaction_date"`
	CreatedAt       time.Time `json:"created_at"`
}

// GetAllTransactions returns all transactions for the authenticated user
// Supports: search (title), filter (type/category/date_from/date_to), sort, pagination
func (tc *TransactionController) GetAllTransactions(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized"})
		return
	}

	query := config.DB.Model(&models.Transaction{}).Where("user_id = ?", userID)

	// Search by title
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		query = query.Where("title LIKE ?", "%"+search+"%")
	}

	// Filter by type
	if txType := strings.TrimSpace(c.Query("type")); txType != "" {
		if txType != string(models.TransactionTypeIncome) && txType != string(models.TransactionTypeExpense) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Type harus income atau expense"})
			return
		}
		query = query.Where("type = ?", txType)
	}

	// Filter by category
	if category := strings.TrimSpace(c.Query("category")); category != "" {
		query = query.Where("category = ?", category)
	}

	// Filter by date range
	dateFrom := strings.TrimSpace(c.Query("date_from"))
	dateTo := strings.TrimSpace(c.Query("date_to"))
	var fromDate, toDate time.Time
	if dateFrom != "" {
		parsed, err := time.Parse("2006-01-02", dateFrom)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "date_from tidak valid"})
			return
		}
		fromDate = parsed
		query = query.Where("transaction_date >= ?", parsed)
	}
	if dateTo != "" {
		parsed, err := time.Parse("2006-01-02", dateTo)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "date_to tidak valid"})
			return
		}
		toDate = parsed.AddDate(0, 0, 1)
		query = query.Where("transaction_date < ?", toDate)
	}
	if dateFrom != "" && dateTo != "" && fromDate.After(toDate.AddDate(0, 0, -1)) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "date_from tidak boleh setelah date_to"})
		return
	}

	// Sort
	switch c.Query("sort") {
	case "oldest":
		query = query.Order("transaction_date ASC")
	case "highest":
		query = query.Order("amount DESC")
	case "lowest":
		query = query.Order("amount ASC")
	default: // latest
		query = query.Order("transaction_date DESC")
	}

	// Count total for pagination
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Gagal menghitung transaksi"})
		return
	}

	// Pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// Fetch transactions
	var transactions []models.Transaction
	if err := query.Offset((page - 1) * pageSize).Limit(pageSize).Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Gagal mengambil transaksi"})
		return
	}

	responses := make([]transactionResponse, len(transactions))
	for i, t := range transactions {
		responses[i] = toTransactionResponse(t)
	}

	totalPages := int64(0)
	if pageSize > 0 {
		totalPages = (total + int64(pageSize) - 1) / int64(pageSize)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Transaksi berhasil diambil",
		"data": gin.H{
			"transactions": responses,
			"meta": gin.H{
				"total":       total,
				"page":        page,
				"page_size":   pageSize,
				"total_pages": totalPages,
			},
		},
	})
}

// GetTransaction returns a specific transaction (only if owned by user)
func (tc *TransactionController) GetTransaction(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "ID tidak valid"})
		return
	}

	var transaction models.Transaction
	if err := config.DB.Where("id = ? AND user_id = ?", id, userID).First(&transaction).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Transaksi tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Transaksi berhasil diambil",
		"data":    toTransactionResponse(transaction),
	})
}

// GetReceipt serves a receipt only when the authenticated user owns the transaction.
func (tc *TransactionController) GetReceipt(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized"})
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "ID tidak valid"})
		return
	}

	var transaction models.Transaction
	if err := config.DB.Where("id = ? AND user_id = ?", id, userID).First(&transaction).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Transaksi tidak ditemukan"})
		return
	}
	if transaction.Receipt == "" || !strings.HasPrefix(strings.ReplaceAll(transaction.Receipt, "\\", "/"), "receipts/") {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Nota tidak ditemukan"})
		return
	}

	clean := filepath.Clean(filepath.FromSlash(transaction.Receipt))
	if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Nota tidak ditemukan"})
		return
	}
	path := filepath.Join("uploads", clean)
	if _, err := os.Stat(path); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "File nota tidak ditemukan"})
		return
	}
	c.Header("Content-Disposition", "inline")
	c.File(path)
}

// CreateTransaction creates a new transaction
// Supports JSON body and multipart/form-data (expense + receipt upload)
func (tc *TransactionController) CreateTransaction(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized"})
		return
	}

	// Determine content type: JSON or multipart
	contentType := c.GetHeader("Content-Type")
	isMultipart := strings.HasPrefix(contentType, "multipart/form-data")

	// Variables to populate
	var (
		txType          string
		title           string
		category        string
		amount          int64
		note            string
		transactionDate = time.Now()
		receiptPath     string
	)

	if isMultipart {
		// ---- multipart/form-data (with optional receipt) ----
		if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Gagal parse form: " + err.Error()})
			return
		}

		txType = strings.ToLower(strings.TrimSpace(c.PostForm("type")))
		title = strings.TrimSpace(c.PostForm("title"))
		category = strings.TrimSpace(c.PostForm("category"))
		note = strings.TrimSpace(c.PostForm("note"))

		amountStr := strings.TrimSpace(c.PostForm("amount"))
		var err error
		amount, err = strconv.ParseInt(amountStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Nominal harus berupa angka"})
			return
		}

		if dateStr := strings.TrimSpace(c.PostForm("transaction_date")); dateStr != "" {
			parsed, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Tanggal tidak valid, gunakan format YYYY-MM-DD"})
				return
			}
			transactionDate = parsed
		}
	} else {
		// ---- JSON body ----
		var body struct {
			Type            string `json:"type"`
			Title           string `json:"title"`
			Category        string `json:"category"`
			Amount          int64  `json:"amount"`
			Note            string `json:"note"`
			TransactionDate string `json:"transaction_date"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Body JSON tidak valid: " + err.Error()})
			return
		}
		txType = strings.ToLower(strings.TrimSpace(body.Type))
		title = strings.TrimSpace(body.Title)
		category = strings.TrimSpace(body.Category)
		note = strings.TrimSpace(body.Note)
		amount = body.Amount

		if body.TransactionDate != "" {
			parsed, err := time.Parse("2006-01-02", body.TransactionDate)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Tanggal tidak valid, gunakan format YYYY-MM-DD"})
				return
			}
			transactionDate = parsed
		}
	}

	// ---- Validate fields ----
	if txType != string(models.TransactionTypeIncome) && txType != string(models.TransactionTypeExpense) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Type harus income atau expense"})
		return
	}
	if title == "" || len([]rune(title)) > 255 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Judul wajib diisi dan maksimal 255 karakter"})
		return
	}
	if len([]rune(note)) > 5000 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Catatan terlalu panjang"})
		return
	}
	if amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Nominal harus lebih dari 0"})
		return
	}
	if !isValidCategory(txType, category) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Kategori tidak valid untuk tipe transaksi ini"})
		return
	}

	// ---- Handle receipt upload (expense only, multipart only) ----
	if isMultipart && txType == string(models.TransactionTypeExpense) {
		file, header, err := c.Request.FormFile("receipt")
		if err == nil {
			defer file.Close()
			receiptPath, err = saveReceipt(file, header)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Upload nota gagal: " + err.Error()})
				return
			}
		}
	}

	// ---- Create transaction ----
	transaction := models.Transaction{
		UserID:          userID,
		Type:            models.TransactionType(txType),
		Title:           title,
		Category:        category,
		Amount:          amount,
		Receipt:         receiptPath,
		Note:            note,
		TransactionDate: transactionDate,
	}
	if err := config.DB.Create(&transaction).Error; err != nil {
		if receiptPath != "" {
			removeReceiptFile(receiptPath)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Gagal membuat transaksi: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Transaksi berhasil dibuat",
		"data":    toTransactionResponse(transaction),
	})
}

// UpdateTransaction updates a transaction (only if owned by user)
// Supports both JSON and multipart/form-data
func (tc *TransactionController) UpdateTransaction(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "ID tidak valid"})
		return
	}

	var transaction models.Transaction
	if err := config.DB.Where("id = ? AND user_id = ?", id, userID).First(&transaction).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Transaksi tidak ditemukan"})
		return
	}

	contentType := c.GetHeader("Content-Type")
	isMultipart := strings.HasPrefix(contentType, "multipart/form-data")

	// Track whether body has values to update
	hasUpdates := false
	oldReceipt := transaction.Receipt
	newReceipt := ""

	if isMultipart {
		if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Gagal parse form: " + err.Error()})
			return
		}

		if v := strings.TrimSpace(c.PostForm("title")); v != "" {
			transaction.Title = v
			hasUpdates = true
		}
		if v := strings.TrimSpace(c.PostForm("category")); v != "" {
			if !isValidCategory(string(transaction.Type), v) {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Kategori tidak valid"})
				return
			}
			transaction.Category = v
			hasUpdates = true
		}
		if v := strings.TrimSpace(c.PostForm("amount")); v != "" {
			amount, err := strconv.ParseInt(v, 10, 64)
			if err != nil || amount <= 0 {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Nominal harus angka lebih dari 0"})
				return
			}
			transaction.Amount = amount
			hasUpdates = true
		}
		if v := strings.TrimSpace(c.PostForm("transaction_date")); v != "" {
			parsed, err := time.Parse("2006-01-02", v)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Tanggal tidak valid"})
				return
			}
			transaction.TransactionDate = parsed
			hasUpdates = true
		}
		if v := c.PostForm("note"); v != "" {
			transaction.Note = v
			hasUpdates = true
		}

		// Handle new receipt upload (replaces old one)
		if transaction.Type == models.TransactionTypeExpense {
			file, header, err := c.Request.FormFile("receipt")
			if err == nil {
				defer file.Close()
				newPath, saveErr := saveReceipt(file, header)
				if saveErr != nil {
					c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Upload nota gagal: " + saveErr.Error()})
					return
				}
				if transaction.Receipt != "" {
					removeReceiptFile(transaction.Receipt)
				}
				transaction.Receipt = newPath
				hasUpdates = true
			}
		}
	} else {
		var body struct {
			Title           *string `json:"title"`
			Category        *string `json:"category"`
			Amount          *int64  `json:"amount"`
			Note            *string `json:"note"`
			TransactionDate *string `json:"transaction_date"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Body JSON tidak valid: " + err.Error()})
			return
		}

		if body.Title != nil {
			transaction.Title = strings.TrimSpace(*body.Title)
			hasUpdates = true
		}
		if body.Category != nil {
			if !isValidCategory(string(transaction.Type), *body.Category) {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Kategori tidak valid"})
				return
			}
			transaction.Category = *body.Category
			hasUpdates = true
		}
		if body.Amount != nil {
			if *body.Amount <= 0 {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Nominal harus lebih dari 0"})
				return
			}
			transaction.Amount = *body.Amount
			hasUpdates = true
		}
		if body.Note != nil {
			transaction.Note = *body.Note
			hasUpdates = true
		}
		if body.TransactionDate != nil && *body.TransactionDate != "" {
			parsed, err := time.Parse("2006-01-02", *body.TransactionDate)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Tanggal tidak valid"})
				return
			}
			transaction.TransactionDate = parsed
			hasUpdates = true
		}
	}

	if transaction.Title == "" || len([]rune(transaction.Title)) > 255 {
		if newReceipt != "" {
			removeReceiptFile(newReceipt)
		}
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Judul wajib diisi dan maksimal 255 karakter"})
		return
	}
	if len([]rune(transaction.Note)) > 5000 {
		if newReceipt != "" {
			removeReceiptFile(newReceipt)
		}
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Catatan terlalu panjang"})
		return
	}
	if !hasUpdates {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Tidak ada field yang diupdate"})
		return
	}

	if err := config.DB.Save(&transaction).Error; err != nil {
		if newReceipt != "" {
			removeReceiptFile(newReceipt)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Gagal mengupdate transaksi"})
		return
	}
	if newReceipt != "" && oldReceipt != "" && oldReceipt != newReceipt {
		removeReceiptFile(oldReceipt)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Transaksi berhasil diupdate",
		"data":    toTransactionResponse(transaction),
	})
}

// DeleteTransaction deletes a transaction (only if owned by user)
func (tc *TransactionController) DeleteTransaction(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "ID tidak valid"})
		return
	}

	var transaction models.Transaction
	if err := config.DB.Where("id = ? AND user_id = ?", id, userID).First(&transaction).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Transaksi tidak ditemukan"})
		return
	}

	// Remove receipt file if exists
	if transaction.Receipt != "" {
		removeReceiptFile(transaction.Receipt)
	}

	if err := config.DB.Delete(&transaction).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Gagal menghapus transaksi"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Transaksi berhasil dihapus",
	})
}

// --- Helpers ---

// isValidCategory checks if a category is valid for the given transaction type
func isValidCategory(txType string, category string) bool {
	if category == "" {
		return false
	}
	allowed := IncomeCategories
	if txType == string(models.TransactionTypeExpense) {
		allowed = ExpenseCategories
	}
	for _, c := range allowed {
		if c == category {
			return true
		}
	}
	return false
}

// saveReceipt saves an uploaded receipt file to disk and returns its relative path
// Validates: extension, MIME type, and max size
func saveReceipt(file multipart.File, header *multipart.FileHeader) (string, error) {
	if header == nil || header.Size <= 0 || header.Size > MaxReceiptSize {
		return "", fmt.Errorf("file terlalu besar atau kosong (maks 5MB)")
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	expectedMIME, ok := allowedImageTypes[ext]
	if !ok {
		return "", fmt.Errorf("format file tidak didukung. Gunakan JPG, JPEG, PNG, atau WebP")
	}

	buffer := make([]byte, 512)
	n, err := io.ReadFull(file, buffer)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return "", fmt.Errorf("gagal membaca file")
	}
	if n == 0 {
		return "", fmt.Errorf("file kosong")
	}
	detectedType := http.DetectContentType(buffer[:n])
	if detectedType != expectedMIME {
		return "", fmt.Errorf("isi file tidak sesuai dengan ekstensi")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("gagal membaca file")
	}

	// Generate unique filename with UUID
	uuidName := uuid.New().String()
	filename := uuidName + ext

	// Ensure directory exists
	dir := filepath.Join("uploads", "receipts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("gagal membuat direktori upload")
	}

	// Save file
	dstPath := filepath.Join(dir, filename)
	dst, err := os.Create(dstPath)
	if err != nil {
		return "", fmt.Errorf("gagal menyimpan file")
	}
	defer dst.Close()

	// file was already rewound to the start above; copy it in one pass.
	// (Do NOT seek again here and do NOT re-write buffer[:n] separately —
	// that previously duplicated the first bytes and corrupted the saved file.)
	written, err := io.Copy(dst, io.LimitReader(file, MaxReceiptSize+1))
	if err != nil {
		_ = os.Remove(dstPath)
		return "", fmt.Errorf("gagal menyimpan file")
	}
	if written > MaxReceiptSize {
		_ = os.Remove(dstPath)
		return "", fmt.Errorf("file terlalu besar (maks 5MB)")
	}

	// Return a logical relative path; the file itself is not publicly served.
	return "receipts/" + filename, nil
}

// removeReceiptFile removes a receipt file from disk (best-effort, path-traversal safe)
func removeReceiptFile(path string) {
	if path == "" {
		return
	}
	// Only allow removing files under receipts/
	cleanPath := strings.ReplaceAll(path, "\\", "/")
	if !strings.HasPrefix(cleanPath, "receipts/") {
		return
	}
	fullPath := filepath.Join("uploads", filepath.FromSlash(cleanPath))
	_ = os.Remove(fullPath)
}

// toTransactionResponse converts a Transaction to the JSON response shape
func toTransactionResponse(t models.Transaction) transactionResponse {
	receiptURL := ""
	if t.Receipt != "" {
		receiptURL = "/api/transactions/" + strconv.FormatUint(uint64(t.ID), 10) + "/receipt"
	}
	return transactionResponse{
		ID:              t.ID,
		Type:            string(t.Type),
		Title:           t.Title,
		Category:        t.Category,
		Amount:          t.Amount,
		Receipt:         t.Receipt,
		ReceiptURL:      receiptURL,
		Note:            t.Note,
		TransactionDate: t.TransactionDate,
		CreatedAt:       t.CreatedAt,
	}
}
