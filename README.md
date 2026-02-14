# OpenClaw Exporter

A Prometheus exporter for monitoring [OpenClaw](https://github.com/openclaw/openclaw) AI assistant metrics.

Monitor your OpenClaw sessions, token usage, costs, and workspace health with Prometheus and Grafana.

## Features

### Session Metrics
Track your AI assistant usage in real-time:
- **Token usage**: input, output, cache read
- **Cost tracking**: cumulative cost in USD
- **Model info**: current provider and model
- **Thinking level**: 0-3 scale
- **Message count**: session activity

### Workspace Metrics
Monitor your OpenClaw workspace:
- **File metrics**: size and modification time for key files
- **Health checks**: workspace file existence
- **Memory tracking**: daily memory files count
- **Skills count**: installed skills from all sources

## Quick Start

### 1. Install Prometheus

**macOS:**
```bash
brew install prometheus
```

**Linux:**
See [Prometheus Installation](https://prometheus.io/docs/prometheus/latest/getting_started/)

### 2. Install OpenClaw Exporter

**Download from GitHub Releases:**
```bash
# macOS (Apple Silicon)
curl -sL https://github.com/JetSquirrel/openclaw_exporter/releases/latest/download/openclaw-exporter-darwin-arm64 -o openclaw_exporter
chmod +x openclaw_exporter
sudo mv openclaw_exporter /usr/local/bin/

# macOS (Intel)
curl -sL https://github.com/JetSquirrel/openclaw_exporter/releases/latest/download/openclaw-exporter-darwin-amd64 -o openclaw_exporter
chmod +x openclaw_exporter
sudo mv openclaw_exporter /usr/local/bin/

# Linux (amd64)
curl -sL https://github.com/JetSquirrel/openclaw_exporter/releases/latest/download/openclaw-exporter-linux-amd64 -o openclaw_exporter
chmod +x openclaw_exporter
sudo mv openclaw_exporter /usr/local/bin/
```

**Or build from source:**
```bash
git clone https://github.com/JetSquirrel/openclaw_exporter.git
cd openclaw_exporter
go build -o openclaw_exporter .
```

### 3. Configure Prometheus

Create `prometheus.yml`:
```yaml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']

  - job_name: 'openclaw'
    static_configs:
      - targets: ['localhost:9101']
    scrape_interval: 5s
```

### 4. Run

```bash
# Start Prometheus
prometheus --config.file=prometheus.yml --storage.tsdb.path=./data

# Start Exporter
openclaw_exporter -openclaw.dir=~/.openclaw/workspace
```

### 5. View Metrics

- **Prometheus UI**: http://localhost:9090
- **Exporter Metrics**: http://localhost:9101/metrics

## Auto-start Services

### macOS (launchd)

Create `~/Library/LaunchAgents/local.openclaw.exporter.plist`:
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>local.openclaw.exporter</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/openclaw_exporter</string>
        <string>-openclaw.dir</string>
        <string>/Users/YOUR_USER/.openclaw/workspace</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/openclaw-exporter.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/openclaw-exporter.log</string>
</dict>
</plist>
```

Create `~/Library/LaunchAgents/local.prometheus.plist`:
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>local.prometheus</string>
    <key>ProgramArguments</key>
    <array>
        <string>/opt/homebrew/bin/prometheus</string>
        <string>--config.file</string>
        <string>/Users/YOUR_USER/.openclaw/workspace/prometheus.yml</string>
        <string>--storage.tsdb.path</string>
        <string>/Users/YOUR_USER/.openclaw/workspace/data</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/prometheus.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/prometheus.log</string>
</dict>
</plist>
```

Load services:
```bash
launchctl load ~/Library/LaunchAgents/local.openclaw.exporter.plist
launchctl load ~/Library/LaunchAgents/local.prometheus.plist
```

### Linux (systemd)

See [SKILL.md](./SKILL.md) for full systemd configuration.

## Available Metrics

### Session Runtime
| Metric | Labels | Description |
|--------|--------|-------------|
| `openclaw_session_active` | agent, session_id | Active session count |
| `openclaw_session_messages_total` | agent, session_id | Total messages |
| `openclaw_session_tokens_input_total` | agent, session_id | Input tokens |
| `openclaw_session_tokens_output_total` | agent, session_id | Output tokens |
| `openclaw_session_tokens_cache_read_total` | agent, session_id | Cache read tokens |
| `openclaw_session_tokens_total` | agent, session_id | Total tokens |
| `openclaw_session_cost_total` | agent, session_id | Total cost (USD) |
| `openclaw_model_info` | agent, session_id, provider, model | Current model |
| `openclaw_thinking_level` | agent, session_id | Thinking level (0-3) |

### Workspace
| Metric | Labels | Description |
|--------|--------|-------------|
| `openclaw_file_size_bytes` | file | File size in bytes |
| `openclaw_file_mtime_seconds` | file | Last modification time |
| `openclaw_workspace_file_exists` | file | File exists (1/0) |
| `openclaw_memory_files_total` | - | Daily memory files count |
| `openclaw_skills_total` | - | Total skills count |
| `openclaw_context_length_total` | - | Context files total size |

## Example PromQL Queries

```promql
# Token usage rate (per minute)
rate(openclaw_session_tokens_total[5m]) * 60

# Cost accumulation over time
openclaw_session_cost_total

# Current model info
openclaw_model_info

# Average tokens per message
openclaw_session_tokens_total / openclaw_session_messages_total

# Workspace health (all files exist?)
sum(openclaw_workspace_file_exists) / count(openclaw_workspace_file_exists)
```

## Command Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-openclaw.dir` | `$OPENCLAW_DIR` | Path to OpenClaw workspace |
| `-openclaw.home` | `~/.openclaw` | Path to OpenClaw home |
| `-web.listen-address` | `:9101` | Listen address |
| `-web.telemetry-path` | `/metrics` | Metrics path |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OPENCLAW_DIR` | - | OpenClaw workspace directory |
| `OPENCLAW_HOME` | `~/.openclaw` | OpenClaw home directory |
| `OPENCLAW_SKILLS_DIR` | `/opt/homebrew/lib/node_modules/openclaw/skills` | System skills directory |

## Example Output

```
# HELP openclaw_session_active Number of active sessions
# TYPE openclaw_session_active gauge
openclaw_session_active{agent="main",session_id="ac610a79-..."} 1

# HELP openclaw_session_cost_total Total cost in USD for session
# TYPE openclaw_session_cost_total gauge
openclaw_session_cost_total{agent="main",session_id="ac610a79-..."} 1.13

# HELP openclaw_session_tokens_total Total tokens used in session
# TYPE openclaw_session_tokens_total gauge
openclaw_session_tokens_total{agent="main",session_id="ac610a79-..."} 4.98e+06

# HELP openclaw_model_info Current model information
# TYPE openclaw_model_info gauge
openclaw_model_info{agent="main",model="z-ai/glm-5",provider="openrouter",session_id="ac610a79-..."} 1

# HELP openclaw_thinking_level Current thinking level
# TYPE openclaw_thinking_level gauge
openclaw_thinking_level{agent="main",session_id="ac610a79-..."} 1

# HELP openclaw_skills_total Total number of skills
# TYPE openclaw_skills_total gauge
openclaw_skills_total 51

# HELP openclaw_file_size_bytes Size of openclaw files in bytes
# TYPE openclaw_file_size_bytes gauge
openclaw_file_size_bytes{file="AGENTS.md"} 7848
openclaw_file_size_bytes{file="SOUL.md"} 5053
openclaw_file_size_bytes{file="MEMORY.md"} 2448
```

## Project Structure

```
.
├── main.go              # HTTP server and entry point
├── collector/
│   ├── collector.go     # Workspace metrics collector
│   └── session_collector.go  # Session runtime metrics collector
├── SKILL.md             # Detailed operation guide
├── README.md
├── go.mod
└── go.sum
```

## License

MIT License

## Links

- [OpenClaw](https://github.com/openclaw/openclaw)
- [Prometheus](https://prometheus.io/)
- [Grafana](https://grafana.com/) (for visualization)
