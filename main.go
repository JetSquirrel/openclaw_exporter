package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/JetSquirrel/openclaw_exporter/collector"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	var (
		listenAddr   = flag.String("web.listen-address", ":9101", "Address to listen on for web interface and telemetry")
		metricsPath  = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics")
		openclawDir  = flag.String("openclaw.dir", os.Getenv("OPENCLAW_DIR"), "Path to openclaw workspace directory")
		openclawHome = flag.String("openclaw.home", os.Getenv("OPENCLAW_HOME"), "Path to openclaw home directory (default: ~/.openclaw)")
	)
	flag.Parse()

	if *openclawDir == "" {
		log.Fatal("openclaw.dir must be specified via flag or OPENCLAW_DIR environment variable")
	}

	// Default openclaw home to ~/.openclaw if not specified
	openclawHomePath := *openclawHome
	if openclawHomePath == "" {
		openclawHomePath = os.Getenv("HOME") + "/.openclaw"
	}

	registry := prometheus.NewRegistry()

	// Register workspace collector
	openclawCollector := collector.NewOpenclawCollector(*openclawDir)
	registry.MustRegister(openclawCollector, openclawCollector.LatencyCollector())

	// Register session collector
	sessionCollector := collector.NewSessionCollector(openclawHomePath)
	registry.MustRegister(sessionCollector)

	http.Handle(*metricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<html>
<head><title>Openclaw Exporter</title></head>
<body>
<h1>Openclaw Exporter</h1>
<p><a href="%s">Metrics</a></p>
</body>
</html>`, *metricsPath)
	})

	log.Printf("Starting openclaw exporter on %s", *listenAddr)
	log.Printf("Workspace: %s, Home: %s", *openclawDir, openclawHomePath)
	if err := http.ListenAndServe(*listenAddr, nil); err != nil {
		log.Fatal(err)
	}
}
