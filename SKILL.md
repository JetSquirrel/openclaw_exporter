---
name: openclaw-exporter
description: Monitor OpenClaw metrics with Prometheus. Export session stats, token usage, costs, and workspace metrics.
homepage: https://github.com/JetSquirrel/openclaw_exporter
metadata:
  {
    "openclaw":
      {
        "emoji": "📊",
        "requires": { "bins": [] },
        "install":
          [
            {
              "id": "prometheus",
              "kind": "brew",
              "formula": "prometheus",
              "bins": ["prometheus"],
              "label": "Install Prometheus via Homebrew",
            },
          ],
      },
  }
---

# OpenClaw Exporter

A Prometheus exporter for monitoring OpenClaw AI assistant metrics including session stats, token usage, costs, and workspace health.

## Installation

### 1. Install Prometheus

**macOS (Homebrew):**
```bash
brew install prometheus
```

**Linux:**
```bash
# Download from https://prometheus.io/download/
wget https://github.com/prometheus/prometheus/releases/download/v3.9.1/prometheus-3.9.1.linux-amd64.tar.gz
tar xvf prometheus-*.tar.gz
sudo mv prometheus-*/prometheus /usr/local/bin/
```

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

## Configuration

### Prometheus Configuration

Create or edit `prometheus.yml`:

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']

  - job_name: 'openclaw'
    static_configs:
      - targets: ['localhost:9101']
    scrape_interval: 5s
```

Save to `~/.openclaw/workspace/prometheus.yml` or any location.

## Running

### Manual Start

**Start Prometheus:**
```bash
prometheus --config.file=~/.openclaw/workspace/prometheus.yml \
           --storage.tsdb.path=~/.openclaw/workspace/data \
           --web.listen-address=:9090
```

**Start OpenClaw Exporter:**
```bash
openclaw_exporter -openclaw.dir=~/.openclaw/workspace \
                  -web.listen-address=:9101
```

**Environment Variables:**
- `OPENCLAW_DIR` - Path to OpenClaw workspace (default: required)
- `OPENCLAW_SKILLS_DIR` - Path to system skills (default: `/opt/homebrew/lib/node_modules/openclaw/skills`)

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
        <string>-web.listen-address</string>
        <string>:9101</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
        <key>OPENCLAW_SKILLS_DIR</key>
        <string>/opt/homebrew/lib/node_modules/openclaw/skills</string>
    </dict>
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
        <string>--web.listen-address</string>
        <string>:9090</string>
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

**Load services:**
```bash
launchctl load ~/Library/LaunchAgents/local.openclaw.exporter.plist
launchctl load ~/Library/LaunchAgents/local.prometheus.plist
```

**Manage services:**
```bash
# Check status
launchctl list | grep -E "(openclaw|prometheus)"

# Stop
launchctl unload ~/Library/LaunchAgents/local.openclaw.exporter.plist
launchctl unload ~/Library/LaunchAgents/local.prometheus.plist

# Start
launchctl load ~/Library/LaunchAgents/local.openclaw.exporter.plist
launchctl load ~/Library/LaunchAgents/local.prometheus.plist

# View logs
tail -f /tmp/openclaw-exporter.log
tail -f /tmp/prometheus.log
```

### Linux (systemd)

Create `/etc/systemd/system/openclaw-exporter.service`:

```ini
[Unit]
Description=OpenClaw Prometheus Exporter
After=network.target

[Service]
Type=simple
User=YOUR_USER
Environment="OPENCLAW_SKILLS_DIR=/opt/openclaw/skills"
ExecStart=/usr/local/bin/openclaw_exporter \
          -openclaw.dir=/home/YOUR_USER/.openclaw/workspace \
          -web.listen-address=:9101
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Create `/etc/systemd/system/prometheus.service`:

```ini
[Unit]
Description=Prometheus
After=network.target

[Service]
Type=simple
User=YOUR_USER
ExecStart=/usr/local/bin/prometheus \
          --config.file=/home/YOUR_USER/.openclaw/workspace/prometheus.yml \
          --storage.tsdb.path=/home/YOUR_USER/.openclaw/workspace/data \
          --web.listen-address=:9090
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

**Enable and start:**
```bash
sudo systemctl daemon-reload
sudo systemctl enable openclaw-exporter prometheus
sudo systemctl start openclaw-exporter prometheus

# Check status
sudo systemctl status openclaw-exporter prometheus
```

## Available Metrics

### Session Metrics
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

### Workspace Metrics
| Metric | Labels | Description |
|--------|--------|-------------|
| `openclaw_file_size_bytes` | file | File size in bytes |
| `openclaw_file_mtime_seconds` | file | Last modification time |
| `openclaw_workspace_file_exists` | file | File exists (1/0) |
| `openclaw_memory_files_total` | - | Daily memory files count |
| `openclaw_skills_total` | - | Total skills count |
| `openclaw_context_length_total` | - | Context files total size |

## Endpoints

- **Prometheus UI**: http://localhost:9090
- **Exporter Metrics**: http://localhost:9101/metrics

## Example Queries

```promql
# Token usage rate (per minute)
rate(openclaw_session_tokens_total[5m]) * 60

# Cost accumulation
openclaw_session_cost_total

# Current model
openclaw_model_info

# Thinking level distribution
openclaw_thinking_level

# Workspace health
openclaw_workspace_file_exists
```

## Troubleshooting

**Exporter shows 0 skills:**
- Set `OPENCLAW_SKILLS_DIR` environment variable
- Default: `/opt/homebrew/lib/node_modules/openclaw/skills`

**Prometheus can't scrape:**
- Check both services are running
- Verify ports 9090 and 9101 are not blocked
- Check logs: `/tmp/prometheus.log` and `/tmp/openclaw-exporter.log`

**Permission denied:**
- Ensure the binary is executable: `chmod +x openclaw_exporter`
- Check file paths in plist/service files

## Links

- [GitHub Repository](https://github.com/JetSquirrel/openclaw_exporter)
- [Prometheus Documentation](https://prometheus.io/docs/)
- [OpenClaw Documentation](https://docs.openclaw.ai)
