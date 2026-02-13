package collector

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// OpenclawCollector collects metrics from openclaw data directory.
type OpenclawCollector struct {
	dir string

	fileSize      *prometheus.Desc
	fileMtime     *prometheus.Desc
	contextLength *prometheus.Desc
	skillsCount   *prometheus.Desc
	agentsCount   *prometheus.Desc
	scrapeSuccess *prometheus.Desc
}

// NewOpenclawCollector creates a new OpenclawCollector.
func NewOpenclawCollector(dir string) *OpenclawCollector {
	return &OpenclawCollector{
		dir: dir,
		fileSize: prometheus.NewDesc(
			"openclaw_file_size_bytes",
			"Size of openclaw files in bytes",
			[]string{"file"}, nil,
		),
		fileMtime: prometheus.NewDesc(
			"openclaw_file_mtime_seconds",
			"Last modification time of openclaw files in seconds since epoch",
			[]string{"file"}, nil,
		),
		contextLength: prometheus.NewDesc(
			"openclaw_context_length_total",
			"Total size of context files in bytes (includes conversation history, tool results, and attachments)",
			nil, nil,
		),
		skillsCount: prometheus.NewDesc(
			"openclaw_skills_total",
			"Total number of skills",
			nil, nil,
		),
		agentsCount: prometheus.NewDesc(
			"openclaw_agents_total",
			"Total number of agents",
			nil, nil,
		),
		scrapeSuccess: prometheus.NewDesc(
			"openclaw_scrape_success",
			"Whether the last scrape was successful",
			nil, nil,
		),
	}
}

// Describe implements prometheus.Collector.
func (c *OpenclawCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.fileSize
	ch <- c.fileMtime
	ch <- c.contextLength
	ch <- c.skillsCount
	ch <- c.agentsCount
	ch <- c.scrapeSuccess
}

// Collect implements prometheus.Collector.
func (c *OpenclawCollector) Collect(ch chan<- prometheus.Metric) {
	success := 1.0

	if err := c.collectFileMetrics(ch); err != nil {
		log.Printf("Error collecting file metrics: %v", err)
		success = 0
	}

	if err := c.collectContextMetrics(ch); err != nil {
		log.Printf("Error collecting context metrics: %v", err)
		success = 0
	}

	if err := c.collectSkillsMetrics(ch); err != nil {
		log.Printf("Error collecting skills metrics: %v", err)
		success = 0
	}

	if err := c.collectAgentsMetrics(ch); err != nil {
		log.Printf("Error collecting agents metrics: %v", err)
		success = 0
	}

	ch <- prometheus.MustNewConstMetric(c.scrapeSuccess, prometheus.GaugeValue, success)
}

func (c *OpenclawCollector) collectFileMetrics(ch chan<- prometheus.Metric) error {
	files := []string{"soul.md", "skill.md", "agent.md"}

	for _, file := range files {
		path := filepath.Join(c.dir, file)
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}

		ch <- prometheus.MustNewConstMetric(
			c.fileSize,
			prometheus.GaugeValue,
			float64(info.Size()),
			file,
		)

		ch <- prometheus.MustNewConstMetric(
			c.fileMtime,
			prometheus.GaugeValue,
			float64(info.ModTime().Unix()),
			file,
		)
	}

	return nil
}

func (c *OpenclawCollector) collectContextMetrics(ch chan<- prometheus.Metric) error {
	contextFiles, err := filepath.Glob(filepath.Join(c.dir, "context*.md"))
	if err != nil {
		return err
	}

	var totalLength int64
	for _, path := range contextFiles {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		totalLength += info.Size()
	}

	ch <- prometheus.MustNewConstMetric(
		c.contextLength,
		prometheus.GaugeValue,
		float64(totalLength),
	)

	return nil
}

func (c *OpenclawCollector) collectSkillsMetrics(ch chan<- prometheus.Metric) error {
	skillPath := filepath.Join(c.dir, "skill.md")
	count, err := countMarkdownSections(skillPath)
	if err != nil {
		if os.IsNotExist(err) {
			count = 0
		} else {
			return err
		}
	}

	ch <- prometheus.MustNewConstMetric(
		c.skillsCount,
		prometheus.GaugeValue,
		float64(count),
	)

	return nil
}

func (c *OpenclawCollector) collectAgentsMetrics(ch chan<- prometheus.Metric) error {
	agentPath := filepath.Join(c.dir, "agent.md")
	count, err := countMarkdownSections(agentPath)
	if err != nil {
		if os.IsNotExist(err) {
			count = 0
		} else {
			return err
		}
	}

	ch <- prometheus.MustNewConstMetric(
		c.agentsCount,
		prometheus.GaugeValue,
		float64(count),
	)

	return nil
}

// countMarkdownSections counts the number of H2 sections (##) in a markdown file.
func countMarkdownSections(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "## ") {
			count++
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, err
	}

	return count, nil
}

// ResponseLatencyCollector tracks response latency metrics.
type ResponseLatencyCollector struct {
	histogram *prometheus.HistogramVec
	startTime time.Time
}

// NewResponseLatencyCollector creates a new ResponseLatencyCollector.
func NewResponseLatencyCollector() *ResponseLatencyCollector {
	return &ResponseLatencyCollector{
		histogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "openclaw_response_duration_seconds",
				Help:    "Response latency in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation"},
		),
		startTime: time.Now(),
	}
}

// Describe implements prometheus.Collector.
func (r *ResponseLatencyCollector) Describe(ch chan<- *prometheus.Desc) {
	r.histogram.Describe(ch)
}

// Collect implements prometheus.Collector.
func (r *ResponseLatencyCollector) Collect(ch chan<- prometheus.Metric) {
	r.histogram.Collect(ch)
}

// ObserveLatency records a latency observation.
func (r *ResponseLatencyCollector) ObserveLatency(operation string, duration time.Duration) {
	r.histogram.WithLabelValues(operation).Observe(duration.Seconds())
}
