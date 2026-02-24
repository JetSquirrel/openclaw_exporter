package collector

import (
	"bufio"
	"context"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Default system skills directory (openclaw npm package location)
const defaultSystemSkillsDir = "/opt/homebrew/lib/node_modules/openclaw/skills"

const (
	defaultScanInterval = 30 * time.Second
	defaultScanTimeout  = 10 * time.Second
)

type fileStat struct {
	name  string
	size  float64
	mtime float64
}

type scrapeSnapshot struct {
	fileStats       []fileStat
	workspaceExists map[string]float64
	contextLength   float64
	skillsCount     float64
	agentsCount     float64
	memoryFiles     float64
	scrapeSuccess   float64
}

// OpenclawCollector collects metrics from openclaw data directory.
type OpenclawCollector struct {
	dir string
	mu  sync.RWMutex

	fileSize         *prometheus.Desc
	fileMtime        *prometheus.Desc
	contextLength    *prometheus.Desc
	skillsCount      *prometheus.Desc
	agentsCount      *prometheus.Desc
	workspaceFiles   *prometheus.Desc
	memoryFilesCount *prometheus.Desc
	scrapeSuccess    *prometheus.Desc
	scanDuration     *prometheus.Desc
	scanErrors       *prometheus.Desc

	scanInterval     time.Duration
	scanTimeout      time.Duration
	latencyCollector *ResponseLatencyCollector
	snapshot         scrapeSnapshot
	lastDuration     float64
	scanErrorsTotal  uint64
}

// NewOpenclawCollector creates a new OpenclawCollector.
func NewOpenclawCollector(dir string) *OpenclawCollector {
	c := &OpenclawCollector{
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
			"Total number of agents (counts agent definitions in agent.md, if present)",
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
		scanDuration: prometheus.NewDesc(
			"openclaw_scan_duration_seconds",
			"Duration of the last background scan in seconds",
			nil, nil,
		),
		scanErrors: prometheus.NewDesc(
			"openclaw_scan_errors_total",
			"Total number of background scan errors",
			nil, nil,
		),
		scanInterval:     defaultScanInterval,
		scanTimeout:      defaultScanTimeout,
		latencyCollector: NewResponseLatencyCollector(),
		snapshot: scrapeSnapshot{
			workspaceExists: make(map[string]float64),
			scrapeSuccess:   0,
		},
	}

	go c.startBackgroundRefresh()

	return c
}

// LatencyCollector exposes the latency collector for registration.
func (c *OpenclawCollector) LatencyCollector() *ResponseLatencyCollector {
	return c.latencyCollector
}

func (c *OpenclawCollector) startBackgroundRefresh() {
	c.refreshSnapshot()

	ticker := time.NewTicker(c.scanInterval)
	for range ticker.C {
		c.refreshSnapshot()
	}
}

func (c *OpenclawCollector) refreshSnapshot() {
	ctx, cancel := context.WithTimeout(context.Background(), c.scanTimeout)
	defer cancel()

	start := time.Now()
	snapshot := scrapeSnapshot{
		workspaceExists: make(map[string]float64),
	}

	errorCount := 0

	if err := c.collectFileMetrics(ctx, &snapshot); err != nil {
		log.Printf("Error collecting file metrics: %v", err)
		errorCount++
	}

	if err := c.collectWorkspaceFileMetrics(ctx, &snapshot); err != nil {
		log.Printf("Error collecting workspace file metrics: %v", err)
		errorCount++
	}

	if err := c.collectContextMetrics(ctx, &snapshot); err != nil {
		log.Printf("Error collecting context metrics: %v", err)
		errorCount++
	}

	if err := c.collectMemoryMetrics(ctx, &snapshot); err != nil {
		log.Printf("Error collecting memory metrics: %v", err)
		errorCount++
	}

	if err := c.collectSkillsMetrics(ctx, &snapshot); err != nil {
		log.Printf("Error collecting skills metrics: %v", err)
		errorCount++
	}

	if err := c.collectAgentsMetrics(ctx, &snapshot); err != nil {
		log.Printf("Error collecting agents metrics: %v", err)
		errorCount++
	}

	snapshot.scrapeSuccess = 1
	if errorCount > 0 {
		snapshot.scrapeSuccess = 0
	}

	duration := time.Since(start)
	c.mu.Lock()
	c.snapshot = snapshot
	c.lastDuration = duration.Seconds()
	if errorCount > 0 {
		c.scanErrorsTotal += uint64(errorCount)
	}
	c.mu.Unlock()

	c.latencyCollector.ObserveLatency("openclaw_scan", duration)
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
	ch <- c.scanDuration
	ch <- c.scanErrors
}

// Collect implements prometheus.Collector.
func (c *OpenclawCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	snapshot := c.snapshot
	duration := c.lastDuration
	scanErrorsTotal := c.scanErrorsTotal
	c.mu.RUnlock()

	for _, stat := range snapshot.fileStats {
		ch <- prometheus.MustNewConstMetric(
			c.fileSize,
			prometheus.GaugeValue,
			stat.size,
			stat.name,
		)

		ch <- prometheus.MustNewConstMetric(
			c.fileMtime,
			prometheus.GaugeValue,
			stat.mtime,
			stat.name,
		)
	}

	for file, exists := range snapshot.workspaceExists {
		ch <- prometheus.MustNewConstMetric(
			c.workspaceFiles,
			prometheus.GaugeValue,
			exists,
			file,
		)
	}

	ch <- prometheus.MustNewConstMetric(
		c.contextLength,
		prometheus.GaugeValue,
		snapshot.contextLength,
	)

	ch <- prometheus.MustNewConstMetric(
		c.skillsCount,
		prometheus.GaugeValue,
		snapshot.skillsCount,
	)

	ch <- prometheus.MustNewConstMetric(
		c.agentsCount,
		prometheus.GaugeValue,
		snapshot.agentsCount,
	)

	ch <- prometheus.MustNewConstMetric(
		c.memoryFilesCount,
		prometheus.GaugeValue,
		snapshot.memoryFiles,
	)

	ch <- prometheus.MustNewConstMetric(
		c.scrapeSuccess,
		prometheus.GaugeValue,
		snapshot.scrapeSuccess,
	)

	ch <- prometheus.MustNewConstMetric(
		c.scanDuration,
		prometheus.GaugeValue,
		duration,
	)

	ch <- prometheus.MustNewConstMetric(
		c.scanErrors,
		prometheus.CounterValue,
		float64(scanErrorsTotal),
	)
}

func (c *OpenclawCollector) collectFileMetrics(ctx context.Context, snapshot *scrapeSnapshot) error {
	// Monitor core workspace files
	// Use a map to track which files we've already seen (case-insensitive)
	// to avoid counting both SOUL.md and soul.md
	files := []string{
		"AGENTS.md", "SOUL.md", "TOOLS.md", "IDENTITY.md",
		"USER.md", "HEARTBEAT.md", "BOOTSTRAP.md", "BOOT.md", "MEMORY.md",
		"soul.md", "skill.md", "agent.md", // legacy files (lowercase)
	}

	// Track which base names we've already reported (lowercase for case-insensitive check)
	reported := make(map[string]bool)

	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return err
		}

		// Get the lowercase version for deduplication check
		fileLower := strings.ToLower(file)

		// Check if we already reported this file (case-insensitive)
		// Skip lowercase version if uppercase version exists
		if reported[fileLower] {
			continue
		}

		path := filepath.Join(c.dir, file)
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}

		// Mark this base name as reported
		reported[fileLower] = true

		snapshot.fileStats = append(snapshot.fileStats, fileStat{
			name:  file,
			size:  float64(info.Size()),
			mtime: float64(info.ModTime().Unix()),
		})
	}

	return nil
}

func (c *OpenclawCollector) collectWorkspaceFileMetrics(ctx context.Context, snapshot *scrapeSnapshot) error {
	// Check existence of key workspace files
	workspaceFiles := []string{
		"AGENTS.md", "SOUL.md", "TOOLS.md", "IDENTITY.md",
		"USER.md", "HEARTBEAT.md", "BOOTSTRAP.md", "MEMORY.md",
	}

	for _, file := range workspaceFiles {
		if err := ctx.Err(); err != nil {
			return err
		}

		path := filepath.Join(c.dir, file)
		exists := 0.0
		if _, err := os.Stat(path); err == nil {
			exists = 1.0
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}

		snapshot.workspaceExists[file] = exists
	}

	return nil
}

func (c *OpenclawCollector) collectMemoryMetrics(ctx context.Context, snapshot *scrapeSnapshot) error {
	// Count daily memory files in memory/ directory
	memoryDir := filepath.Join(c.dir, "memory")
	count := 0
	if err := ctx.Err(); err != nil {
		return err
	}

	entries, err := os.ReadDir(memoryDir)
	if err != nil {
		if os.IsNotExist(err) {
			snapshot.memoryFiles = 0
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".md" {
			count++
		}
	}

	snapshot.memoryFiles = float64(count)

	return nil
}

func (c *OpenclawCollector) collectContextMetrics(ctx context.Context, snapshot *scrapeSnapshot) error {
	contextFiles, err := filepath.Glob(filepath.Join(c.dir, "context*.md"))
	if err != nil {
		return err
	}

	var totalLength int64
	for _, path := range contextFiles {
		if err := ctx.Err(); err != nil {
			return err
		}

		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		totalLength += info.Size()
	}

	snapshot.contextLength = float64(totalLength)

	return nil
}

func (c *OpenclawCollector) collectSkillsMetrics(ctx context.Context, snapshot *scrapeSnapshot) error {
	totalCount := 0
	if err := ctx.Err(); err != nil {
		return err
	}

	// Check legacy skill.md file for H2 sections
	skillPath := filepath.Join(c.dir, "skill.md")
	if count, err := countMarkdownSections(skillPath); err == nil {
		totalCount += count
	}

	// Check workspace skills/ directory for SKILL.md files
	skillsDir := filepath.Join(c.dir, "skills")
	if entries, err := os.ReadDir(skillsDir); err == nil {
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}

			if entry.IsDir() {
				skillFile := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
				if _, err := os.Stat(skillFile); err == nil {
					totalCount++
				}
			}
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Check user skills directory at ~/.openclaw/skills
	homeDir := os.Getenv("HOME")
	if homeDir != "" {
		userSkillsDir := filepath.Join(homeDir, ".openclaw", "skills")
		if entries, err := os.ReadDir(userSkillsDir); err == nil {
			for _, entry := range entries {
				if err := ctx.Err(); err != nil {
					return err
				}

				if entry.IsDir() {
					skillFile := filepath.Join(userSkillsDir, entry.Name(), "SKILL.md")
					if _, err := os.Stat(skillFile); err == nil {
						totalCount++
					}
				}
			}
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	// Check system skills directory (openclaw npm package)
	// Can be overridden via OPENCLAW_SKILLS_DIR environment variable
	systemSkillsDir := resolveSystemSkillsDir()
	if systemSkillsDir != "" {
		if entries, err := os.ReadDir(systemSkillsDir); err == nil {
			for _, entry := range entries {
				if err := ctx.Err(); err != nil {
					return err
				}

				if entry.IsDir() {
					skillFile := filepath.Join(systemSkillsDir, entry.Name(), "SKILL.md")
					if _, err := os.Stat(skillFile); err == nil {
						totalCount++
					}
				}
			}
		} else if !os.IsNotExist(err) {
			return err
		}
	}

	snapshot.skillsCount = float64(totalCount)

	return nil
}

func (c *OpenclawCollector) collectAgentsMetrics(ctx context.Context, snapshot *scrapeSnapshot) error {
	totalCount := 0
	if err := ctx.Err(); err != nil {
		return err
	}

	// Check legacy agent.md file for agent definitions (H2 sections)
	// Note: AGENTS.md is a workspace configuration document, not an agent list
	agentPath := filepath.Join(c.dir, "agent.md")
	if count, err := countMarkdownSections(agentPath); err == nil {
		totalCount += count
	}

	snapshot.agentsCount = float64(totalCount)

	return nil
}

func resolveSystemSkillsDir() string {
	if systemSkillsDir := os.Getenv("OPENCLAW_SKILLS_DIR"); systemSkillsDir != "" {
		return systemSkillsDir
	}

	if runtime.GOOS == "darwin" {
		if _, err := os.Stat(defaultSystemSkillsDir); err == nil {
			return defaultSystemSkillsDir
		}
	}

	return ""
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
		if strings.HasPrefix(line, "##") {
			rest := strings.TrimLeft(line[2:], " \t")
			if rest != "" {
				count++
			}
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
