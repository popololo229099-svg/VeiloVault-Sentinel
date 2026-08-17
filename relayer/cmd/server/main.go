// Package main is the entry point for the Veilo Relayer service.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gagliardetto/solana-go"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"

	"github.com/popolo229099-svg/veilo-relayer/internal/infrastructure/cache"
	"github.com/popolo229099-svg/veilo-relayer/internal/infrastructure/database"
	solanaClient "github.com/popolo229099-svg/veilo-relayer/internal/infrastructure/solana"
	"github.com/popolo229099-svg/veilo-relayer/internal/interfaces/api"
	"github.com/popolo229099-svg/veilo-relayer/internal/usecase"
)

func main() {
	// Load configuration
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	// Set defaults
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("solana.rpc", "https://api.mainnet-beta.solana.com")
	viper.SetDefault("solana.ws", "wss://api.mainnet-beta.solana.com")
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", "5432")
	viper.SetDefault("database.name", "veilo_relayer")
	viper.SetDefault("database.user", "postgres")
	viper.SetDefault("database.password", "postgres")
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", "6379")
	viper.SetDefault("relayer.fee_bps", 50)
	viper.SetDefault("relayer.min_fee", 1000000)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			panic(fmt.Errorf("read config: %w", err))
		}
	}

	// Initialize logger
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		Level(zerolog.InfoLevel).
		With().
		Timestamp().
		Str("service", "veilo-relayer").
		Logger()

	logger.Info().Msg("starting veilo relayer service")

	// Initialize database
	dbDSN := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		viper.GetString("database.host"),
		viper.GetInt("database.port"),
		viper.GetString("database.user"),
		viper.GetString("database.password"),
		viper.GetString("database.name"),
	)

	db, err := database.NewPostgresDB(dbDSN)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer db.Close()

	// Run migrations
	if err := db.Migrate(); err != nil {
		logger.Fatal().Err(err).Msg("failed to run migrations")
	}

	// Initialize cache
	redisAddr := fmt.Sprintf("%s:%d",
		viper.GetString("redis.host"),
		viper.GetInt("redis.port"),
	)
	redisCache := cache.NewRedisCache(redisAddr, "", 0)
	defer redisCache.Close()

	// Initialize Solana client
	solClient := solanaClient.NewClient(
		viper.GetString("solana.rpc"),
		viper.GetString("solana.ws"),
		logger,
	)

	// Initialize repositories
	txRepo := database.NewTransactionRepository(db)
	poolRepo := database.NewPoolRepository(db)
	relayerRepo := database.NewRelayerRepository(db)

	// Initialize event bus (simplified - in production use NATS or RabbitMQ)
	eventBus := &SimpleEventBus{logger: logger}

	// Load relayer keypair
	keypairStr := viper.GetString("relayer.private_key")
	var keypair *solana.Wallet
	if keypairStr != "" {
		key, err := solana.PrivateKeyFromBase58(keypairStr)
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to parse relayer private key")
		}
		wallet := solana.Wallet{PrivateKey: key}
		keypair = &wallet
	} else {
		// Generate temporary keypair for development
		keypair = solana.NewWallet()
		logger.Warn().Str("pubkey", keypair.PublicKey().String()).Msg("using generated keypair - set RELAYER_PRIVATE_KEY for production")
	}

	// Initialize use cases
	relayUC := usecase.NewRelayUseCase(
		txRepo,
		poolRepo,
		relayerRepo,
		redisCache,
		solClient,
		eventBus,
		keypair,
		logger,
		uint16(viper.GetInt("relayer.fee_bps")),
		uint64(viper.GetInt("relayer.min_fee")),
	)

	// Initialize router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(api.CORSMiddleware())
	router.Use(api.LoggerMiddleware())

	// Register routes
	handler := api.NewHandler(relayUC)
	handler.RegisterRoutes(router)

	// Health check
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service":   "veilo-relayer",
			"version":   "1.0.0",
			"timestamp": time.Now().Unix(),
		})
	})

	// Start server
	addr := fmt.Sprintf("%s:%s", viper.GetString("server.host"), viper.GetString("server.port"))
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start in goroutine
	go func() {
		logger.Info().Str("addr", addr).Msg("server starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("server failed")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal().Err(err).Msg("server forced to shutdown")
	}

	logger.Info().Msg("server exited")
}

// SimpleEventBus is a simple event bus implementation.
type SimpleEventBus struct {
	logger zerolog.Logger
}

func (b *SimpleEventBus) Publish(topic string, message interface{}) error {
	b.logger.Info().Str("topic", topic).Msg("event published")
	return nil
}

func (b *SimpleEventBus) Subscribe(topic string, handler func(interface{})) error {
	b.logger.Info().Str("topic", topic).Msg("subscribed to topic")
	return nil
}
