package main

import (
	rootassets "backend"
	"backend/internal/config"
	coreclock "backend/internal/core/clock"
	coredb "backend/internal/core/database"
	authapp "backend/internal/modules/auth/application"
	authhttp "backend/internal/modules/auth/delivery/http"
	jwtinfra "backend/internal/modules/auth/infrastructure/jwt"
	tokenpg "backend/internal/modules/auth/infrastructure/postgres"
	mealapp "backend/internal/modules/meals/application"
	mealhttp "backend/internal/modules/meals/delivery/http"
	mealpg "backend/internal/modules/meals/infrastructure/postgres"
	userapp "backend/internal/modules/users/application"
	userhttp "backend/internal/modules/users/delivery/http"
	userpg "backend/internal/modules/users/infrastructure/postgres"
	workforceapp "backend/internal/modules/workforce/application"
	workforcehttp "backend/internal/modules/workforce/delivery/http"
	workforcepg "backend/internal/modules/workforce/infrastructure/postgres"
	"context"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancelApp := context.WithCancel(context.Background())
	defer cancelApp()
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err = db.Ping(ctx); err != nil {
		log.Fatal(err)
	}
	if err = coredb.RunMigrations(ctx, db, rootassets.Assets, "migrations"); err != nil {
		log.Fatal(err)
	}
	usersRepo := userpg.New(db)
	tokensRepo := tokenpg.New(db)
	serverClock := coreclock.NewAdjustable()
	clock := serverClock.Now
	jwtService := jwtinfra.New(cfg.JWTSecret)
	usersHandler := userhttp.New(userapp.NewService(usersRepo))
	authHandler := authhttp.New(authapp.NewService(usersRepo, tokensRepo, jwtService, cfg.AccessTTL, cfg.WorkerAccessTTL, cfg.RefreshTTL))
	workforceHandler := workforcehttp.New(workforceapp.NewService(workforcepg.New(db), usersRepo, clock))
	mealBroker := mealhttp.NewBroker(cfg.AllowedOrigins)
	mealService := mealapp.NewService(mealpg.New(db), clock)
	mealHandler := mealhttp.New(mealService, mealBroker)
	go runMealScheduler(ctx, mealService, cfg.MealSchedulerInterval, cfg.MealSchedulerLookback)
	r := gin.Default()
	_ = r.SetTrustedProxies(nil)
	r.Use(authhttp.LimitRequestBody(1 << 20))
	r.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.AllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	r.GET("/openapi.yaml", func(c *gin.Context) {
		content, readErr := rootassets.Assets.ReadFile("docs/openapi.yaml")
		if readErr != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Data(http.StatusOK, "application/yaml; charset=utf-8", content)
	})
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/openapi.yaml")))
	v1 := r.Group("/api/v1")
	authLimit := authhttp.RateLimit(10, time.Minute)
	v1.POST("/auth/bootstrap/admin", authLimit, usersHandler.BootstrapAdmin)
	v1.POST("/auth/login/password", authLimit, authHandler.LoginPassword)
	v1.POST("/auth/login/dni", authLimit, authHandler.LoginDNI)
	v1.POST("/auth/refresh", authLimit, authHandler.Refresh)
	v1.PUT("/test/server-time", func(c *gin.Context) {
		var request struct {
			Datetime string `json:"datetime"`
		}
		if c.ShouldBindJSON(&request) != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
			return
		}
		value, parseErr := time.Parse(time.RFC3339, request.Datetime)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "datetime is required and must use RFC3339, for example 2026-09-04T12:30:00-05:00"})
			return
		}
		serverClock.Set(value)
		now := serverClock.Now().In(peruLocation())
		log.Printf("[TEST_CLOCK] server time adjusted datetime=%s timezone=America/Lima", now.Format(time.RFC3339))
		c.JSON(http.StatusOK, gin.H{
			"datetime": now.Format(time.RFC3339),
			"date":     now.Format("2006-01-02"),
			"time":     now.Format("15:04:05"),
			"timezone": "America/Lima",
			"adjusted": serverClock.Adjusted(),
		})
	})
	v1.GET("/test/server-time", authhttp.RequireAuth(jwtService), authhttp.RequireRoles("RRHH", "OWNER", "COLLABORATOR", "WORKER"), func(c *gin.Context) {
		now := serverClock.Now().In(peruLocation())
		c.JSON(http.StatusOK, gin.H{
			"datetime": now.Format(time.RFC3339),
			"date":     now.Format("2006-01-02"),
			"time":     now.Format("15:04:05"),
			"timezone": "America/Lima",
			"adjusted": serverClock.Adjusted(),
		})
	})
	protected := v1.Group("")
	protected.Use(authhttp.RequireAuth(jwtService))
	protected.POST("/users/register/management", authhttp.RequireRoles("ADMIN"), usersHandler.RegisterManagement)
	protected.POST("/users/register/collaborator", authhttp.RequireRoles("ADMIN", "OWNER"), usersHandler.RegisterCollaborator)
	protected.POST("/users/register/worker", authhttp.RequireRoles("ADMIN", "RRHH"), workforceHandler.RegisterWorker)
	protected.POST("/worker-shift-assignments", authhttp.RequireRoles("RRHH"), workforceHandler.AssignWorker)
	protected.POST("/worker-shift-assignments/add-massive", authhttp.RequireRoles("RRHH"), workforceHandler.AddMassiveShiftWorkers)
	protected.PUT("/worker-shift-assignments/:id", authhttp.RequireRoles("RRHH"), workforceHandler.UpdateAssignment)
	protected.DELETE("/worker-shift-assignments/:id", authhttp.RequireRoles("RRHH"), workforceHandler.DeleteAssignment)
	protected.GET("/workers/:id/shifts/range", authhttp.RequireRoles("RRHH"), workforceHandler.ListWorkerAssignmentsRange)
	protected.GET("/workers/my/shifts/range", authhttp.RequireRoles("WORKER"), workforceHandler.ListMyAssignmentsRange)
	protected.GET("/meal-claims/my/preview", authhttp.RequireRoles("WORKER"), mealHandler.ClaimPreview)
	protected.POST("/meal-claims/my/confirm-print", authhttp.RequireRoles("WORKER"), mealHandler.ConfirmPrint)
	protected.GET("/meal-orders", authhttp.RequireRoles("COLLABORATOR"), mealHandler.ListOrders)
	protected.GET("/meal-orders/:id", authhttp.RequireRoles("COLLABORATOR"), mealHandler.GetOrder)
	protected.PUT("/meal-orders/:id/validate", authhttp.RequireRoles("COLLABORATOR"), mealHandler.ValidateOrder)
	// Kept as a compatibility alias for clients using the previous contract.
	protected.PATCH("/meal-orders/:id/validate", authhttp.RequireRoles("COLLABORATOR"), mealHandler.ValidateOrder)
	protected.GET("/collaborator/meal-orders", authhttp.RequireRoles("COLLABORATOR"), mealHandler.ListOrders)
	protected.GET("/collaborator/meal-orders/:id", authhttp.RequireRoles("COLLABORATOR"), mealHandler.GetOrder)
	protected.PUT("/collaborator/meal-orders/:id/validate", authhttp.RequireRoles("COLLABORATOR"), mealHandler.ValidateOrder)
	protected.GET("/collaborator/meal-status-reports/daily", authhttp.RequireRoles("COLLABORATOR"), mealHandler.DailyMealStatusReport)
	protected.GET("/meal-status-reports", authhttp.RequireRoles("OWNER"), mealHandler.MealStatusReport)
	protected.GET("/meal-status-reports/export.xlsx", authhttp.RequireRoles("OWNER"), mealHandler.ExportMealStatusReport)
	protected.GET("/workforce/shift-preview", authhttp.RequireRoles("OWNER"), workforceHandler.ShiftPreview)
	protected.GET("/workforce/shift-preview/export.xlsx", authhttp.RequireRoles("OWNER"), workforceHandler.ExportShiftPreview)
	protected.GET("/meal-schedules", authhttp.RequireRoles("WORKER"), mealHandler.ListSchedules)
	protected.GET("/workers/my/status", authhttp.RequireRoles("WORKER"), mealHandler.WorkerStatus)
	protected.GET("/users/my", usersHandler.My)
	protected.GET("/users", authhttp.RequireRoles("ADMIN"), usersHandler.Users)
	protected.PATCH("/users/my/password", usersHandler.ChangePassword)
	protected.PUT("/users/:id/password/reset", authhttp.RequireRoles("ADMIN"), usersHandler.ResetPassword)
	protected.GET("/users/workers", authhttp.RequireRoles("RRHH"), usersHandler.Workers)
	protected.GET("/users/collaborators", authhttp.RequireRoles("OWNER"), usersHandler.Collaborators)
	v1.GET("/ws/meal-orders", authhttp.RequireWebSocketAuth(jwtService), authhttp.RequireRoles("COLLABORATOR"), mealHandler.OrdersWebSocket)
	v1.GET("/collaborator/ws/meal-orders", authhttp.RequireWebSocketAuth(jwtService), authhttp.RequireRoles("COLLABORATOR"), mealHandler.OrdersWebSocket)
	server := &http.Server{Addr: ":" + cfg.Port, Handler: r, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		log.Printf("API http://localhost:%s | Swagger http://localhost:%s/swagger/index.html", cfg.Port, cfg.Port)
		if e := server.ListenAndServe(); e != nil && e != http.ErrServerClosed {
			log.Fatal(e)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	cancelApp()
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdown)
}

func peruLocation() *time.Location {
	location, err := time.LoadLocation("America/Lima")
	if err != nil {
		return time.FixedZone("America/Lima", -5*60*60)
	}
	return location
}

func runMealScheduler(ctx context.Context, service *mealapp.Service, interval time.Duration, lookbackDays int) {
	run := func() {
		created, err := service.CloseExpiredMealWindows(ctx, lookbackDays)
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("meal scheduler error: %v", err)
			}
			return
		}
		if created > 0 {
			log.Printf("meal scheduler finalized %d meal records", created)
		}
	}
	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
