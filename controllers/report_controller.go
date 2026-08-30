package controllers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/nabungyuk/nabungyuk/middleware"
	"github.com/nabungyuk/nabungyuk/models"
)

// ReportController handles report-related requests
type ReportController struct{}

// NewReportController creates a new ReportController
func NewReportController() *ReportController {
	return &ReportController{}
}

// reportChartItem is one slice of the expense-distribution doughnut chart
type reportChartItem struct {
	Category string `json:"category"`
	Amount   int64  `json:"amount"`
}

// GetReport returns the all-time financial summary (total income & expense) for the user
func (rc *ReportController) GetReport(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized"})
		return
	}

	totalIncome := sumByType(userID, models.TransactionTypeIncome)
	totalExpense := sumByType(userID, models.TransactionTypeExpense)
	totalSaving := sumAllDeposits(userID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Laporan berhasil diambil",
		"data": gin.H{
			"total_income":  totalIncome,
			"total_expense": totalExpense,
			"total_saving":  totalSaving,
			"balance":       totalIncome - totalExpense - totalSaving,
		},
	})
}

// GetReportChart returns expense distribution by category (current month) for the doughnut chart
func (rc *ReportController) GetReportChart(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized"})
		return
	}

	labels, values := expenseByCategory(userID, time.Now())

	data := make([]reportChartItem, len(labels))
	for i := range labels {
		data[i] = reportChartItem{Category: labels[i], Amount: values[i]}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Data grafik laporan berhasil diambil",
		"data":    data,
	})
}
