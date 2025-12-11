package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fsanano/go-test/internal/config"
	"fsanano/go-test/internal/handler"
	"fsanano/go-test/internal/repository"
	"fsanano/go-test/internal/service"
	"fsanano/go-test/internal/service/skinport"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	// 1. Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Setup Database
	ctx := context.Background()
	dbPool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbPool.Close()

	if err := dbPool.Ping(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	fmt.Println("Connected to database")

	// 3. Setup Logic
	// Logic - Shop
	shopRepo := repository.NewShopRepository(dbPool)
	shopService := service.NewShopService(shopRepo)
	shopHandler := handler.NewShopHandler(shopService)

	// Logic - Skinport
	skinportClient := skinport.NewClient(skinport.Config{
		APIURL:   cfg.Skinport.APIURL,
		ClientID: cfg.Skinport.ClientID,
		APIKey:   cfg.Skinport.APIKey,
	})

	h := handler.NewHandler(skinportClient, shopHandler)

	// 4. Setup Server
	server := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: h,
	}

	// 5. Run Server with Graceful Shutdown
	go func() {
		fmt.Printf("Starting server on port %s\n", cfg.ServerPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 2)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("Shutting down server...")

	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	fmt.Println("Server exiting")
}
