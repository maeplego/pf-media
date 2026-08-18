package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/portfolio/pf-media/api/internal/auth"
	"github.com/portfolio/pf-media/api/internal/config"
	"github.com/portfolio/pf-media/api/internal/objectstore"
	"github.com/portfolio/pf-media/api/internal/queue"
	"github.com/portfolio/pf-media/api/internal/service"
	"github.com/portfolio/pf-media/api/internal/store/postgres"
	"github.com/portfolio/pf-media/api/internal/telemetry"
	"github.com/portfolio/pf-media/api/internal/web"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	cfg, err := config.FromEnv()
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	otelEP := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	svcName := strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME"))
	if svcName == "" {
		svcName = "media-api"
	}
	shutdownTel, err := telemetry.Init(ctx, svcName, otelEP)
	if err != nil {
		log.Fatal(err)
	}

	store, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	s3Client, err := objectstore.New(cfg.S3Endpoint, cfg.S3PublicEndpoint, cfg.S3Region, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Bucket, cfg.S3UseSSL)
	if err != nil {
		log.Fatal(err)
	}
	if err := s3Client.EnsureBucket(ctx); err != nil {
		log.Fatal(err)
	}

	var q *queue.Redis
	if cfg.RedisURL != "" {
		q, err = queue.New(cfg.RedisURL)
		if err != nil {
			log.Fatal(err)
		}
		if err := q.Ping(ctx); err != nil {
			log.Fatal(err)
		}
	}

	media := service.NewMedia(
		store,
		service.NewObjectStore(s3Client),
		q,
		cfg.QuotaBytes,
		cfg.MaxUploadBytes,
		time.Duration(cfg.PresignTTL)*time.Second,
	)

	mw := auth.New(cfg.DevAuth, cfg.OIDCIssuer, cfg.OIDCInternalBase, cfg.OIDCAudience)
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           otelhttp.NewHandler(web.New(media).Routes(mw, cfg.ProcessorToken), "media-api"),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("media api listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shCtx)
	_ = shutdownTel(shCtx)
}
