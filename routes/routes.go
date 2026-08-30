package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/nabungyuk/nabungyuk/controllers"
	"github.com/nabungyuk/nabungyuk/middleware"
)

// SetupRoutes initializes all routes
func SetupRoutes(router *gin.Engine) {
	// Public routes
	public := router.Group("/api")
	{
		public.POST("/register", controllers.NewAuthController().Register)
		public.POST("/login", middleware.RateLimitLogin(), controllers.NewAuthController().Login)
	}

	// Protected routes
	protected := router.Group("/api")
	protected.Use(middleware.AuthMiddleware())
	{
		// Auth
		protected.POST("/logout", controllers.NewAuthController().Logout)
		protected.GET("/profile", controllers.NewAuthController().GetProfile)

		// Dashboard
		dashboard := controllers.NewDashboardController()
		protected.GET("/dashboard", dashboard.GetDashboard)
		protected.GET("/dashboard/chart", dashboard.GetChart)

		// Transactions
		transaction := controllers.NewTransactionController()
		protected.GET("/transactions", transaction.GetAllTransactions)
		protected.POST("/transactions", transaction.CreateTransaction)
		protected.GET("/transactions/:id", transaction.GetTransaction)
		protected.GET("/transactions/:id/receipt", transaction.GetReceipt)
		protected.PUT("/transactions/:id", transaction.UpdateTransaction)
		protected.DELETE("/transactions/:id", transaction.DeleteTransaction)

		// Savings
		saving := controllers.NewSavingController()
		protected.GET("/savings", saving.GetAllSavingGoals)
		protected.POST("/savings", saving.CreateSavingGoal)
		protected.GET("/savings/:id", saving.GetSavingGoal)
		protected.PUT("/savings/:id", saving.UpdateSavingGoal)
		protected.DELETE("/savings/:id", saving.DeleteSavingGoal)
		protected.POST("/savings/:id/deposit", saving.AddDeposit)
		protected.GET("/savings/:id/deposits", saving.GetDeposits)

		// Reminders
		reminder := controllers.NewReminderController()
		protected.GET("/reminders", reminder.GetAllReminders)
		protected.POST("/reminders", reminder.CreateReminder)
		protected.GET("/reminders/:id", reminder.GetReminder)
		protected.PUT("/reminders/:id", reminder.UpdateReminder)
		protected.DELETE("/reminders/:id", reminder.DeleteReminder)

		// Reports
		report := controllers.NewReportController()
		protected.GET("/reports", report.GetReport)
		protected.GET("/reports/chart", report.GetReportChart)
	}
}
