package main

import (
	"backend/internal/config"
	coredb "backend/internal/core/database"
	authapp "backend/internal/modules/auth/application"
	authhttp "backend/internal/modules/auth/delivery/http"
	jwtinfra "backend/internal/modules/auth/infrastructure/jwt"
	tokenpg "backend/internal/modules/auth/infrastructure/postgres"
	userapp "backend/internal/modules/users/application"
	userhttp "backend/internal/modules/users/delivery/http"
	userpg "backend/internal/modules/users/infrastructure/postgres"
	workforceapp "backend/internal/modules/workforce/application"
	workforcehttp "backend/internal/modules/workforce/delivery/http"
	workforcepg "backend/internal/modules/workforce/infrastructure/postgres"
	"context"
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
	ctx := context.Background()
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err = db.Ping(ctx); err != nil {
		log.Fatal(err)
	}
	if err = coredb.RunMigrations(ctx, db, "migrations"); err != nil {
		log.Fatal(err)
	}
	usersRepo := userpg.New(db)
	tokensRepo := tokenpg.New(db)
	jwtService := jwtinfra.New(cfg.JWTSecret, cfg.AccessTTL)
	usersHandler := userhttp.New(userapp.NewService(usersRepo))
	authHandler := authhttp.New(authapp.NewService(usersRepo, tokensRepo, jwtService, cfg.RefreshTTL))
	workforceHandler := workforcehttp.New(workforceapp.NewService(workforcepg.New(db), usersRepo))
	r := gin.Default()
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	r.StaticFile("/openapi.yaml", "./docs/openapi.yaml")
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/openapi.yaml")))
	v1 := r.Group("/api/v1")
	v1.POST("/auth/bootstrap/admin", usersHandler.BootstrapAdmin)
	v1.POST("/auth/login/password", authHandler.LoginPassword)
	v1.POST("/auth/login/dni", authHandler.LoginDNI)
	v1.POST("/auth/refresh", authHandler.Refresh)
	protected := v1.Group("")
	protected.Use(authhttp.RequireAuth(jwtService))
	protected.POST("/users/register/management", authhttp.RequireRoles("ADMIN"), usersHandler.RegisterManagement)
	protected.POST("/users/register/worker", authhttp.RequireRoles("ADMIN", "OWNER", "RRHH"), workforceHandler.RegisterWorker)
	protected.POST("/shifts", authhttp.RequireRoles("ADMIN", "OWNER", "RRHH"), workforceHandler.CreateShift)
	protected.GET("/shifts", workforceHandler.ListShifts)
	protected.POST("/worker-shift-assignments", authhttp.RequireRoles("ADMIN", "OWNER", "RRHH"), workforceHandler.AssignWorker)
	protected.GET("/worker-shift-assignments", authhttp.RequireRoles("ADMIN", "OWNER", "RRHH"), workforceHandler.ListAssignments)
	protected.GET("/users", usersHandler.List)
	protected.GET("/users/:id", usersHandler.Get)
	server := &http.Server{Addr: ":" + cfg.Port, Handler: r, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Printf("API http://localhost:%s | Swagger http://localhost:%s/swagger/index.html", cfg.Port, cfg.Port)
		if e := server.ListenAndServe(); e != nil && e != http.ErrServerClosed {
			log.Fatal(e)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdown)
}
