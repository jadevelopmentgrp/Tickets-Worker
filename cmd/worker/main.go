package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"cloud.google.com/go/profiler"
	archiverclient "github.com/jadevelopmentgrp/Tickets-Archiver-Client"
	"github.com/jadevelopmentgrp/Tickets-Utilities/observability"
	"github.com/jadevelopmentgrp/Tickets-Utilities/rpc"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/blacklist"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/cache"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/dbclient"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/integrations"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/listeners/messagequeue"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/metrics/prometheus"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/metrics/statsd"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/redis"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/rpc/listeners"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/utils"
	"github.com/jadevelopmentgrp/Tickets-Worker/config"
	"github.com/jadevelopmentgrp/Tickets-Worker/event"
	"github.com/jadevelopmentgrp/Tickets-Worker/i18n"
	"github.com/rxdn/gdl/rest/request"
	"go.uber.org/zap"
)

func main() {
	go func() {
		fmt.Println(http.ListenAndServe(":6060", nil))
	}()

	config.Parse()

	if config.Conf.CloudProfiler.Enabled {
		cfg := profiler.Config{
			Service:        utils.GetServiceName(),
			ServiceVersion: "1.0.0",
			ProjectID:      config.Conf.CloudProfiler.ProjectId,
		}

		if err := profiler.Start(cfg); err != nil {
			fmt.Printf("Failed to start the profiler: %v", err)
		}
	}

	logger, err := observability.Configure(config.Conf.JsonLogs, config.Conf.LogLevel)
	if err != nil {
		panic(err)
	}

	logger.Info("Connecting to Redis")
	if err := redis.Connect(); err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
		return
	}

	logger.Info("Connected to Redis")

	logger.Info("Connecting to DB")
	dbclient.Connect()
	logger.Info("Connected to DB")

	logger.Info("Loading i18n files")
	i18n.Init()
	logger.Info("Loaded i18n files")

	logger.Info("Connecting to cache")
	pgCache, err := cache.Connect(logger.With(zap.String("service", "cache")))
	if err != nil {
		logger.Fatal("Failed to connect to cache", zap.Error(err))
		return
	}

	cache.Client = &pgCache
	logger.Info("Connected to cache")

	logger.Info("Connecting to clickhouse")
	dbclient.ConnectAnalytics(logger.With(zap.String("service", "clickhouse")))
	logger.Info("Connected to clickhouse")

	// Configure HTTP proxy
	if config.Conf.Discord.ProxyUrl != "" {
		logger.Info("Configuring REST proxy", zap.String("url", config.Conf.Discord.ProxyUrl))
		request.Client.Timeout = config.Conf.Discord.RequestTimeout
		request.RegisterPreRequestHook(utils.ProxyHook)
	}

	logger.Info("Configuring microservice clients (no I/O)")

	utils.ArchiverClient = archiverclient.NewArchiverClient(
		archiverclient.NewProxyRetriever(config.Conf.Archiver.Url),
		[]byte(config.Conf.Archiver.AesKey),
	)

	logger.Info("Starting Prometheus server")
	prometheus.StartServer(config.Conf.Prometheus.Address)
	logger.Info("Started Prometheus server")

	logger.Info("Starting StatsD client")
	statsd.Client, err = statsd.NewClient(config.Conf.Statsd.Address, config.Conf.Statsd.Prefix)
	if err != nil {
		logger.Error("Failed to start StatsD client", zap.Error(err))
	} else {
		request.RegisterPreRequestHook(statsd.RestHook)
		go statsd.Client.StartDaemon()
		logger.Info("Started StatsD client")
	}

	logger.Info("Registering Prometheus hooks")
	request.RegisterPreRequestHook(prometheus.PreRequestHook)
	request.RegisterPostRequestHook(prometheus.PostRequestHook)

	logger.Info("Initialising integrations")
	integrations.InitIntegrations()

	go messagequeue.ListenTicketClose()
	go messagequeue.ListenAutoClose()
	go messagequeue.ListenCloseRequestTimer()

	go blacklist.StartCacheRefreshLoop(logger.With(zap.String("service", "blacklist_refresh")))

	if config.Conf.WorkerMode == config.WorkerModeInteractions {
		logger.Info("Starting HTTP server", zap.String("mode", string(config.Conf.WorkerMode)))

		event.HttpListen(redis.Client, &pgCache)
	} else if config.Conf.WorkerMode == config.WorkerModeGateway {
		logger.Info("Starting event listeners", zap.String("mode", string(config.Conf.WorkerMode)))

		go event.HttpListen(redis.Client, &pgCache)

		var wg sync.WaitGroup

		rpcClient, err := rpc.NewClient(
			logger.With(zap.String("service", "rpc")),
			rpc.Config{
				Brokers:             config.Conf.Kafka.Brokers,
				ConsumerGroup:       "worker",
				ConsumerConcurrency: config.Conf.Kafka.GoroutineLimit,
			},
			map[string]rpc.Listener{
				// Listen for gateway events over Kafka
				config.Conf.Kafka.EventsTopic: event.NewKafkaListener(
					logger.With(zap.String("service", "gateway-events-kafka")),
					&pgCache,
				),
				// TODO: Don't hardcode
				"tickets.rpc.categoryupdate": listeners.NewTicketStatusUpdater(&pgCache, logger),
			})

		if err != nil {
			logger.Fatal("Failed to create RPC client", zap.Error(err))
			return
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			rpcClient.StartConsumer()
		}()

		shutdownCh := make(chan os.Signal, 1)
		signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)
		<-shutdownCh

		logger.Info("Received shutdown signal")
		rpcClient.Shutdown()

		if waitTimeout(&wg, time.Second*10) {
			logger.Info("Shutdown completed gracefully")
		} else {
			logger.Warn("Graceful shutdown timed out, exiting now")
		}
	} else {
		logger.Fatal("Invalid worker mode", zap.String("mode", string(config.Conf.WorkerMode)))
	}
}

func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	ch := make(chan struct{})
	go func() {
		defer close(ch)
		wg.Wait()
	}()

	select {
	case <-ch:
		return true
	case <-time.After(timeout):
		return false
	}
}
