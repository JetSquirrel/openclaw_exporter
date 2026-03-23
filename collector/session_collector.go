package collector

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// Default openclaw home directory
const defaultOpenclawHome = "/.openclaw"

// SessionCollector collects runtime session metrics from openclaw.
type SessionCollector struct {
	openclawHome string

	// Session info
	sessionActive     *prometheus.Desc
	sessionMessages   *prometheus.Desc
	sessionUpdated    *prometheus.Desc

	// Token usage
	sessionTokensInput     *prometheus.Desc
	sessionTokensOutput    *prometheus.Desc
	sessionTokensCacheRead *prometheus.Desc
	sessionTokensCacheWrite *prometheus.Desc
	sessionTokensTotal     *prometheus.Desc

	// Cost
	sessionCostTotal *prometheus.Desc

	// Cache efficiency
	cacheHitRate *prometheus.Desc

	// Session duration
	sessionDuration *prometheus.Desc

	// Error tracking
	sessionErrors *prometheus.Desc

	// Model info
	modelInfo *prometheus.Desc

	// Thinking level
	thinkingLevel *prometheus.Desc

	// Scrape success
	scrapeSuccess *prometheus.Desc
}

// NewSessionCollector creates a new SessionCollector.
func NewSessionCollector(openclawHome string) *SessionCollector {
	if openclawHome == "" {
		openclawHome = os.Getenv("HOME") + defaultOpenclawHome
	}

	return &SessionCollector{
		openclawHome: openclawHome,
		sessionActive: prometheus.NewDesc(
			"openclaw_session_active",
			"Number of active sessions",
			[]string{"agent", "session_id"}, nil,
		),
		sessionMessages: prometheus.NewDesc(
			"openclaw_session_messages_total",
			"Total number of messages in current session",
			[]string{"agent", "session_id"}, nil,
		),
		sessionUpdated: prometheus.NewDesc(
			"openclaw_session_updated_timestamp",
			"Last update timestamp of session",
			[]string{"agent", "session_id"}, nil,
		),
		sessionTokensInput: prometheus.NewDesc(
			"openclaw_session_tokens_input_total",
			"Total input tokens used in session",
			[]string{"agent", "session_id"}, nil,
		),
		sessionTokensOutput: prometheus.NewDesc(
			"openclaw_session_tokens_output_total",
			"Total output tokens used in session",
			[]string{"agent", "session_id"}, nil,
		),
		sessionTokensCacheRead: prometheus.NewDesc(
			"openclaw_session_tokens_cache_read_total",
			"Total cache read tokens in session",
			[]string{"agent", "session_id"}, nil,
		),
		sessionTokensCacheWrite: prometheus.NewDesc(
			"openclaw_session_tokens_cache_write_total",
			"Total cache write tokens in session",
			[]string{"agent", "session_id"}, nil,
		),
		sessionTokensTotal: prometheus.NewDesc(
			"openclaw_session_tokens_total",
			"Total tokens used in session (input + output + cache)",
			[]string{"agent", "session_id"}, nil,
		),
		sessionCostTotal: prometheus.NewDesc(
			"openclaw_session_cost_total",
			"Total cost in USD for session",
			[]string{"agent", "session_id"}, nil,
		),
		modelInfo: prometheus.NewDesc(
			"openclaw_model_info",
			"Current model information",
			[]string{"agent", "session_id", "provider", "model"}, nil,
		),
		thinkingLevel: prometheus.NewDesc(
			"openclaw_thinking_level",
			"Current thinking level (0=off, 1=low, 2=medium, 3=high)",
			[]string{"agent", "session_id"}, nil,
		),
		cacheHitRate: prometheus.NewDesc(
			"openclaw_session_cache_hit_rate",
			"Cache hit rate (cache_read / (cache_read + cache_write))",
			[]string{"agent", "session_id"}, nil,
		),
		sessionDuration: prometheus.NewDesc(
			"openclaw_session_duration_seconds",
			"Session duration in seconds",
			[]string{"agent", "session_id"}, nil,
		),
		sessionErrors: prometheus.NewDesc(
			"openclaw_session_errors_total",
			"Total errors encountered in session",
			[]string{"agent", "session_id"}, nil,
		),
		scrapeSuccess: prometheus.NewDesc(
			"openclaw_session_scrape_success",
			"Whether session scrape was successful",
			[]string{"agent"}, nil,
		),
	}
}

// Describe implements prometheus.Collector.
func (c *SessionCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.sessionActive
	ch <- c.sessionMessages
	ch <- c.sessionUpdated
	ch <- c.sessionTokensInput
	ch <- c.sessionTokensOutput
	ch <- c.sessionTokensCacheRead
	ch <- c.sessionTokensCacheWrite
	ch <- c.sessionTokensTotal
	ch <- c.sessionCostTotal
	ch <- c.cacheHitRate
	ch <- c.sessionDuration
	ch <- c.sessionErrors
	ch <- c.modelInfo
	ch <- c.thinkingLevel
	ch <- c.scrapeSuccess
}

// Collect implements prometheus.Collector.
func (c *SessionCollector) Collect(ch chan<- prometheus.Metric) {
	agentsDir := filepath.Join(c.openclawHome, "agents")

	// List agent directories
	agentEntries, err := os.ReadDir(agentsDir)
	if err != nil {
		log.Printf("Error reading agents directory: %v", err)
		ch <- prometheus.MustNewConstMetric(c.scrapeSuccess, prometheus.GaugeValue, 0, "unknown")
		return
	}

	for _, agentEntry := range agentEntries {
		if !agentEntry.IsDir() {
			continue
		}
		agentName := agentEntry.Name()

		sessionsFile := filepath.Join(agentsDir, agentName, "sessions", "sessions.json")
		if _, err := os.Stat(sessionsFile); err != nil {
			continue
		}

		c.collectAgentSessions(ch, agentName, sessionsFile)
	}
}

// sessionsJSON represents the sessions.json structure
type sessionsJSON map[string]struct {
	SessionID      string `json:"sessionId"`
	UpdatedAt      int64  `json:"updatedAt"`
	SessionFile    string `json:"sessionFile"`
	CompactionCount int   `json:"compactionCount"`
}

// sessionEvent represents an event in the session jsonl file
type sessionEvent struct {
	Type           string `json:"type"`
	ID             string `json:"id"`
	Provider       string `json:"provider"`
	ModelID        string `json:"modelId"`
	ThinkingLevel  string `json:"thinkingLevel"`
	Error          *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error"`
	Message        *struct {
		Provider string `json:"provider"`
		Model    string `json:"model"`
		Usage    *struct {
			Input       int     `json:"input"`
			Output      int     `json:"output"`
			CacheRead   int     `json:"cacheRead"`
			CacheWrite  int     `json:"cacheWrite"`
			TotalTokens int     `json:"totalTokens"`
			Cost        *struct {
				Total float64 `json:"total"`
			} `json:"cost"`
		} `json:"usage"`
	} `json:"message"`
}

func (c *SessionCollector) collectAgentSessions(ch chan<- prometheus.Metric, agentName, sessionsFile string) {
	// Read sessions.json
	data, err := os.ReadFile(sessionsFile)
	if err != nil {
		log.Printf("Error reading sessions.json for agent %s: %v", agentName, err)
		return
	}

	var sessions sessionsJSON
	if err := json.Unmarshal(data, &sessions); err != nil {
		log.Printf("Error parsing sessions.json for agent %s: %v", agentName, err)
		return
	}

	for key, session := range sessions {
		// Only process "agent:main:main" style keys (active sessions)
		if !strings.HasPrefix(key, "agent:") {
			continue
		}

		sessionID := session.SessionID
		if sessionID == "" {
			continue
		}

		// Session active
		ch <- prometheus.MustNewConstMetric(
			c.sessionActive,
			prometheus.GaugeValue,
			1,
			agentName, sessionID,
		)

		// Session updated timestamp
		ch <- prometheus.MustNewConstMetric(
			c.sessionUpdated,
			prometheus.GaugeValue,
			float64(session.UpdatedAt/1000), // Convert ms to seconds
			agentName, sessionID,
		)

		// Parse session file for detailed metrics
		if session.SessionFile != "" {
			c.collectSessionFileMetrics(ch, agentName, sessionID, session.SessionFile)
		}
	}

	ch <- prometheus.MustNewConstMetric(c.scrapeSuccess, prometheus.GaugeValue, 1, agentName)
}

func (c *SessionCollector) collectSessionFileMetrics(ch chan<- prometheus.Metric, agentName, sessionID, sessionFile string) {
	file, err := os.Open(sessionFile)
	if err != nil {
		log.Printf("Error opening session file %s: %v", sessionFile, err)
		return
	}
	defer file.Close()

	var (
		messageCount       int
		totalInputTokens   int
		totalOutputTokens  int
		totalCacheRead     int
		totalCacheWrite    int
		totalCost          float64
		currentProvider    string
		currentModel       string
		thinkingLevelNum   float64
		errorCount         int
		firstTimestamp     int64
		lastTimestamp      int64
	)

	scanner := bufio.NewScanner(file)
	// Increase buffer size for large lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}

		var event sessionEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		switch event.Type {
		case "message":
			messageCount++
			if event.Message != nil {
				// Get model from message
				if event.Message.Model != "" {
					currentModel = event.Message.Model
				}
				if event.Message.Provider != "" {
					currentProvider = event.Message.Provider
				}
				// Get usage
				if event.Message.Usage != nil {
					totalInputTokens += event.Message.Usage.Input
					totalOutputTokens += event.Message.Usage.Output
					totalCacheRead += event.Message.Usage.CacheRead
					totalCacheWrite += event.Message.Usage.CacheWrite
					if event.Message.Usage.Cost != nil {
						totalCost += event.Message.Usage.Cost.Total
					}
				}
			}
			// Track errors in messages
			if event.Error != nil {
				errorCount++
			}

		case "model_change":
			if event.ModelID != "" {
				currentModel = event.ModelID
			}
			if event.Provider != "" {
				currentProvider = event.Provider
			}

		case "thinking_level_change":
			switch event.ThinkingLevel {
			case "off":
				thinkingLevelNum = 0
			case "low":
				thinkingLevelNum = 1
			case "medium":
				thinkingLevelNum = 2
			case "high":
				thinkingLevelNum = 3
			}

		case "session_start":
			// Track session start time (timestamp is in event.ID typically)
			if firstTimestamp == 0 {
				firstTimestamp = lastTimestamp
			}

		case "error":
			errorCount++
		}

		// Track timestamps for duration calculation (if present in future)
		// For now, we'll use message count as a proxy
		lastTimestamp++
	}

	// Report metrics
	ch <- prometheus.MustNewConstMetric(
		c.sessionMessages,
		prometheus.GaugeValue,
		float64(messageCount),
		agentName, sessionID,
	)

	ch <- prometheus.MustNewConstMetric(
		c.sessionTokensInput,
		prometheus.GaugeValue,
		float64(totalInputTokens),
		agentName, sessionID,
	)

	ch <- prometheus.MustNewConstMetric(
		c.sessionTokensOutput,
		prometheus.GaugeValue,
		float64(totalOutputTokens),
		agentName, sessionID,
	)

	ch <- prometheus.MustNewConstMetric(
		c.sessionTokensCacheRead,
		prometheus.GaugeValue,
		float64(totalCacheRead),
		agentName, sessionID,
	)

	ch <- prometheus.MustNewConstMetric(
		c.sessionTokensCacheWrite,
		prometheus.GaugeValue,
		float64(totalCacheWrite),
		agentName, sessionID,
	)

	ch <- prometheus.MustNewConstMetric(
		c.sessionTokensTotal,
		prometheus.GaugeValue,
		float64(totalInputTokens+totalOutputTokens+totalCacheRead+totalCacheWrite),
		agentName, sessionID,
	)

	ch <- prometheus.MustNewConstMetric(
		c.sessionCostTotal,
		prometheus.GaugeValue,
		totalCost,
		agentName, sessionID,
	)

	// Cache hit rate
	totalCache := totalCacheRead + totalCacheWrite
	cacheHitRate := 0.0
	if totalCache > 0 {
		cacheHitRate = float64(totalCacheRead) / float64(totalCache)
	}
	ch <- prometheus.MustNewConstMetric(
		c.cacheHitRate,
		prometheus.GaugeValue,
		cacheHitRate,
		agentName, sessionID,
	)

	// Session duration (approximate based on message count for now)
	// In a real implementation, this would use actual timestamps
	sessionDuration := float64(messageCount) * 30.0 // Approximate 30s per message
	ch <- prometheus.MustNewConstMetric(
		c.sessionDuration,
		prometheus.GaugeValue,
		sessionDuration,
		agentName, sessionID,
	)

	// Error count
	ch <- prometheus.MustNewConstMetric(
		c.sessionErrors,
		prometheus.GaugeValue,
		float64(errorCount),
		agentName, sessionID,
	)

	// Model info (value=1 for info metric)
	if currentModel != "" {
		ch <- prometheus.MustNewConstMetric(
			c.modelInfo,
			prometheus.GaugeValue,
			1,
			agentName, sessionID, currentProvider, currentModel,
		)
	}

	// Thinking level
	ch <- prometheus.MustNewConstMetric(
		c.thinkingLevel,
		prometheus.GaugeValue,
		thinkingLevelNum,
		agentName, sessionID,
	)
}
