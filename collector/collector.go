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

	fileSize         *prometheus.Desc
	fileMtime        *prometheus.Desc
	contextLength    *prometheus.Desc
	skillsCount      *prometheus.Desc
	agentsCount      *prometheus.Desc
	workspaceFiles   *prometheus.Desc
	memoryFilesCount *prometheus.Desc
	scrapeSuccess    *prometheus.Desc
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
			"Total number of skills in workspace and managed directories",
			nil, nil,
		),
		agentsCount: prometheus.NewDesc(
			"openclaw_agents_total",
			"Total number of agents",
			nil, nil,
		),
		workspaceFiles: prometheus.NewDesc(
			"openclaw_workspace_file_exists",
			"Whether workspace files exist (AGENTS.md, SOUL.md, TOOLS.md, IDENTITY.md, USER.md, HEARTBEAT.md, BOOTSTRAP.md, MEMORY.md)",
			[]string{"file"}, nil,
		),
		memoryFilesCount: prometheus.NewDesc(
			"openclaw_memory_files_total",
			"Total number of daily memory files in memory/ directory",
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
	ch <- c.workspaceFiles
	ch <- c.memoryFilesCount
	ch <- c.scrapeSuccess
}

// Collect implements prometheus.Collector.
func (c *OpenclawCollector) Collect(ch chan<- prometheus.Metric) {
	success := 1.0

	if err := c.collectFileMetrics(ch); err != nil {
		log.Printf("Error collecting file metrics: %v", err)
		success = 0
	}

	if err := c.collectWorkspaceFileMetrics(ch); err != nil {
		log.Printf("Error collecting workspace file metrics: %v", err)
		success = 0
	}

	if err := c.collectContextMetrics(ch); err != nil {
		log.Printf("Error collecting context metrics: %v", err)
		success = 0
	}

	if err := c.collectMemoryMetrics(ch); err != nil {
		log.Printf("Error collecting memory metrics: %v", err)
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
	// Monitor core workspace files
	files := []string{
		"AGENTS.md", "SOUL.md", "TOOLS.md", "IDENTITY.md",
		"USER.md", "HEARTBEAT.md", "BOOTSTRAP.md", "BOOT.md", "MEMORY.md",
		"soul.md", "skill.md", "agent.md", // legacy files
	}

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

func (c *OpenclawCollector) collectWorkspaceFileMetrics(ch chan<- prometheus.Metric) error {
	// Check existence of key workspace files
	workspaceFiles := []string{
		"AGENTS.md", "SOUL.md", "TOOLS.md", "IDENTITY.md",
		"USER.md", "HEARTBEAT.md", "BOOTSTRAP.md", "MEMORY.md",
	}

	for _, file := range workspaceFiles {
		path := filepath.Join(c.dir, file)
		exists := 0.0
		if _, err := os.Stat(path); err == nil {
			exists = 1.0
		}

		ch <- prometheus.MustNewConstMetric(
			c.workspaceFiles,
			prometheus.GaugeValue,
			exists,
			file,
		)
	}

	return nil
}

func (c *OpenclawCollector) collectMemoryMetrics(ch chan<- prometheus.Metric) error {
	// Count daily memory files in memory/ directory
	memoryDir := filepath.Join(c.dir, "memory")
	count := 0

	if entries, err := os.ReadDir(memoryDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".md" {
				count++
			}
		}
	}

	ch <- prometheus.MustNewConstMetric(
		c.memoryFilesCount,
		prometheus.GaugeValue,
		float64(count),
	)

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
	totalCount := 0

	// Check legacy skill.md file for H2 sections
	skillPath := filepath.Join(c.dir, "skill.md")
	if count, err := countMarkdownSections(skillPath); err == nil {
		totalCount += count
	}

	// Check workspace skills/ directory for SKILL.md files
	skillsDir := filepath.Join(c.dir, "skills")
	if entries, err := os.ReadDir(skillsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				skillFile := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
				if _, err := os.Stat(skillFile); err == nil {
					totalCount++
				}
			}
		}
	}

	// Check managed skills directory at ~/.openclaw/skills if OPENCLAW_HOME is set
	openclawHome := os.Getenv("HOME")
	if openclawHome != "" {
		managedSkillsDir := filepath.Join(openclawHome, ".openclaw", "skills")
		if entries, err := os.ReadDir(managedSkillsDir); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					skillFile := filepath.Join(managedSkillsDir, entry.Name(), "SKILL.md")
					if _, err := os.Stat(skillFile); err == nil {
						totalCount++
					}
				}
			}
		}
	}

	ch <- prometheus.MustNewConstMetric(
		c.skillsCount,
		prometheus.GaugeValue,
		float64(totalCount),
	)

	return nil
}

func (c *OpenclawCollector) collectAgentsMetrics(ch chan<- prometheus.Metric) error {
	totalCount := 0

	// Check legacy agent.md file
	agentPath := filepath.Join(c.dir, "agent.md")
	if count, err := countMarkdownSections(agentPath); err == nil {
		totalCount += count
	}

	// Check AGENTS.md file
	agentsPath := filepath.Join(c.dir, "AGENTS.md")
	if count, err := countMarkdownSections(agentsPath); err == nil {
		totalCount += count
	}

	ch <- prometheus.MustNewConstMetric(
		c.agentsCount,
		prometheus.GaugeValue,
		float64(totalCount),
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
