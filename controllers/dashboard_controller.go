package controllers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/nabungyuk/nabungyuk/config"
	"github.com/nabungyuk/nabungyuk/middleware"
	"github.com/nabungyuk/nabungyuk/models"
)

// DashboardController handles dashboard-related requests
type DashboardController struct{}

// NewDashboardController creates a new DashboardController
func NewDashboardController() *DashboardController {
	return &DashboardController{}
}

// GetDashboard returns dashboard summary
// balance = income - expense - saving
func (dc *DashboardController) GetDashboard(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized"})
		return
	}

	// Total income
	income := sumByType(userID, models.TransactionTypeIncome)
	// Total expense
	expense := sumByType(userID, models.TransactionTypeExpense)
	// Total saving (sum of all deposits)
	saving := sumAllDeposits(userID)

	balance := income - expense - saving

	// Recent transactions
	var recent []models.Transaction
	config.DB.Where("user_id = ?", userID).
		Order("transaction_date DESC").
		Limit(8).
		Find(&recent)

	recentResponses := make([]transactionResponse, len(recent))
	for i, t := range recent {
		recentResponses[i] = toTransactionResponse(t)
	}

	// Saving goals
	var goals []models.SavingGoal
	config.DB.Where("user_id = ?", userID).Order("created_at DESC").Find(&goals)
	goalResponses := make([]savingGoalResponse, len(goals))
	for i, g := range goals {
		goalResponses[i] = toSavingGoalResponse(g, sumDeposits(g.ID))
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Dashboard berhasil dimuat",
		"data": gin.H{
			"balance":             balance,
			"income":              income,
			"expense":             expense,
			"saving":              saving,
			"recent_transactions": recentResponses,
			"saving_goals":        goalResponses,
		},
	})
}

// GetChart returns chart data ready for Chart.js
func (dc *DashboardController) GetChart(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized"})
		return
	}

	// 1. Income vs Expense per month (last 6 months)
	monthlyLabels, monthlyIncome, monthlyExpense := monthlyIncomeExpense(userID, 6)

	// 2. Expense by category (current month)
	categoryLabels, categoryValues := expenseByCategory(userID, time.Now())

	// 3. Saving progress (per month, last 6 months)
	savingLabels, savingValues := monthlySaving(userID, 6)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Data grafik berhasil dimuat",
		"data": gin.H{
			"monthly": gin.H{
				"labels":  monthlyLabels,
				"income":  monthlyIncome,
				"expense": monthlyExpense,
			},
			"expense_categories": gin.H{
				"labels": categoryLabels,
				"values": categoryValues,
			},
			"saving": gin.H{
				"labels": savingLabels,
				"values": savingValues,
			},
		},
	})
}

// --- Helpers ---

// sumByType computes SUM(amount) for a transaction type for a user
func sumByType(userID uint, txType models.TransactionType) int64 {
	var sum int64
	config.DB.Model(&models.Transaction{}).
		Where("user_id = ? AND type = ?", userID, txType).
		Select("COALESCE(SUM(amount),0)").
		Scan(&sum)
	return sum
}

// sumAllDeposits computes the total saving deposits for a user
func sumAllDeposits(userID uint) int64 {
	var sum int64
	config.DB.Model(&models.SavingDeposit{}).
		Joins("JOIN saving_goals ON saving_goals.id = saving_deposits.saving_goal_id").
		Where("saving_goals.user_id = ?", userID).
		Select("COALESCE(SUM(saving_deposits.amount),0)").
		Scan(&sum)
	return sum
}

// monthlyIncomeExpense returns labels and income/expense series for the last n months
func monthlyIncomeExpense(userID uint, n int) ([]string, []int64, []int64) {
	labels := make([]string, n)
	income := make([]int64, n)
	expense := make([]int64, n)

	now := time.Now()
	for i := n - 1; i >= 0; i-- {
		month := now.AddDate(0, -i, 0)
		idx := n - 1 - i
		labels[idx] = month.Format("Jan 06")

		start := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.Local)
		end := start.AddDate(0, 1, 0)

		income[idx] = sumByTypeRange(userID, models.TransactionTypeIncome, start, end)
		expense[idx] = sumByTypeRange(userID, models.TransactionTypeExpense, start, end)
	}
	return labels, income, expense
}

// sumByTypeRange computes SUM(amount) for a type within a date range
func sumByTypeRange(userID uint, txType models.TransactionType, start, end time.Time) int64 {
	var sum int64
	config.DB.Model(&models.Transaction{}).
		Where("user_id = ? AND type = ? AND transaction_date >= ? AND transaction_date < ?",
			userID, txType, start, end).
		Select("COALESCE(SUM(amount),0)").
		Scan(&sum)
	return sum
}

// expenseByCategory returns expense totals grouped by category for a given month
func expenseByCategory(userID uint, ref time.Time) ([]string, []int64) {
	start := time.Date(ref.Year(), ref.Month(), 1, 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 1, 0)

	type catRow struct {
		Category string
		Total    int64
	}
	var rows []catRow
	config.DB.Model(&models.Transaction{}).
		Select("category, COALESCE(SUM(amount),0) as total").
		Where("user_id = ? AND type = ? AND transaction_date >= ? AND transaction_date < ?",
			userID, models.TransactionTypeExpense, start, end).
		Group("category").
		Order("total DESC").
		Scan(&rows)

	labels := make([]string, len(rows))
	values := make([]int64, len(rows))
	for i, r := range rows {
		labels[i] = r.Category
		values[i] = r.Total
	}
	return labels, values
}

// monthlySaving returns total deposits per month for the last n months
func monthlySaving(userID uint, n int) ([]string, []int64) {
	labels := make([]string, n)
	values := make([]int64, n)

	now := time.Now()
	for i := n - 1; i >= 0; i-- {
		month := now.AddDate(0, -i, 0)
		idx := n - 1 - i
		labels[idx] = month.Format("Jan 06")

		start := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.Local)
		end := start.AddDate(0, 1, 0)

		var sum int64
		config.DB.Model(&models.SavingDeposit{}).
			Joins("JOIN saving_goals ON saving_goals.id = saving_deposits.saving_goal_id").
			Where("saving_goals.user_id = ? AND saving_deposits.deposit_date >= ? AND saving_deposits.deposit_date < ?",
				userID, start, end).
			Select("COALESCE(SUM(saving_deposits.amount),0)").
			Scan(&sum)
		values[idx] = sum
	}
	return labels, values
}
