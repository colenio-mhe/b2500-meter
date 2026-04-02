package main

import (
	"b2500-meter-go/internal/config"
	"b2500-meter-go/pkg/emulator"
	"b2500-meter-go/pkg/provider"
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	setupLogging(cfg.LogLevel)

	multiProvider := setupProviders(cfg)
	handler := setupHandler(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	startServers(ctx, handler, multiProvider)

	<-ctx.Done()
	slog.Info("Shutting down...")
}

func setupLogging(level string) {
	var l slog.Level
	switch strings.ToLower(level) {
	case "debug":
		l = slog.LevelDebug
	case "info", "":
		l = slog.LevelInfo
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: l,
	})
	slog.SetDefault(slog.New(handler))
}

func setupProviders(cfg config.Config) provider.PowerProvider {
	var providers []provider.PowerProvider

	if len(cfg.Providers) > 0 {
		for _, pc := range cfg.Providers {
			var p provider.PowerProvider
			switch pc.Type {
			case "tasmota":
				p = provider.NewTasmotaProvider(
					pc.IP,
					pc.User,
					pc.Password,
					pc.Status,
					pc.Payload,
					pc.Label,
					pc.LabelIn,
					pc.LabelOut,
					pc.Calculate,
				)
				slog.Info("Added Tasmota provider", "ip", pc.IP)
			case "mock":
				p = provider.NewMockProvider(pc.Power)
				slog.Info("Added Mock provider", "power", pc.Power)
			case "mqtt":
				mqttP, mqttErr := provider.NewMqttProvider(
					pc.Broker,
					pc.Port,
					pc.Topic,
					pc.User,
					pc.Password,
					pc.JsonPath,
				)
				if mqttErr != nil {
					slog.Error("Failed to initialize MQTT provider", "error", mqttErr)
					os.Exit(1)
				}
				slog.Info("Waiting for first MQTT message...", "topic", pc.Topic)
				if err := mqttP.WaitForMessage(5 * time.Second); err != nil {
					slog.Warn("Did not receive MQTT message within timeout", "topic", pc.Topic, "error", err)
				}
				p = mqttP
				slog.Info("Added MQTT provider", "broker", pc.Broker, "topic", pc.Topic)
			default:
				slog.Error("Unknown provider type", "type", pc.Type)
				os.Exit(1)
			}

			if pc.Throttle > 0 {
				interval := time.Duration(pc.Throttle * float64(time.Second))
				p = provider.NewThrottledProvider(p, interval)
				slog.Info("Throttling enabled", "interval", pc.Throttle)
			}
			providers = append(providers, p)
		}
	} else {
		slog.Info("No providers configured, using default mock provider (0.00W)")
		providers = append(providers, provider.NewMockProvider(0))
	}

	return provider.NewMultiProvider(providers)
}

func setupHandler(cfg config.Config) emulator.DeviceHandler {
	var handler emulator.DeviceHandler

	deviceID := cfg.DeviceID
	if deviceID == "" {
		deviceID = "shellypro3em-1234567890ab"
	}

	switch cfg.Device {
	case "shellypro3em", "":
		handler = &emulator.ShellyPro3EMHandler{
			DeviceID: deviceID,
		}
		if cfg.Device == "" {
			slog.Info("No device type configured, defaulting to shellypro3em")
		} else {
			slog.Info("Using device emulator", "device", cfg.Device, "id", deviceID)
		}
	default:
		slog.Error("Unknown device type", "device", cfg.Device)
		os.Exit(1)
	}
	return handler
}

func startServers(ctx context.Context, handler emulator.DeviceHandler, p provider.PowerProvider) {
	var wg sync.WaitGroup
	ports := []int{1010, 2220}
	for _, port := range ports {
		srv := &emulator.Server{
			Port:    port,
			Handler: handler,
			Power:   p,
		}

		wg.Add(1)
		go func(s *emulator.Server) {
			defer wg.Done()
			if err := s.Run(ctx); err != nil {
				slog.Error("Server failed", "port", s.Port, "error", err)
			}
		}(srv)
	}

	slog.Info("B2500 emulator running. Switch the Marstek battery to 'Auto' mode.", "ports", ports)

	go func() {
		<-ctx.Done()
		slog.Info("Shutting down servers...")
		wg.Wait()
		slog.Info("Shutdown complete.")
	}()
}
