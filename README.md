# Openclaw Exporter

A Prometheus exporter for monitoring [openclaw](https://deepwiki.com/openclaw) personal AI assistant metrics.

## Features

This exporter provides the following metrics:

### File Metrics
- `openclaw_file_size_bytes` - Size of openclaw files (soul.md, skill.md, agent.md) in bytes
- `openclaw_file_mtime_seconds` - Last modification time of openclaw files in seconds since epoch

### Context Metrics
- `openclaw_context_length_total` - Total length of context window in characters (sum of all context*.md files)

### Counts
- `openclaw_skills_total` - Total number of skills (H2 sections in skill.md)
- `openclaw_agents_total` - Total number of agents (H2 sections in agent.md)

### Response Latency
- `openclaw_response_duration_seconds` - Response latency histogram (extensible for future use)

### Health
- `openclaw_scrape_success` - Whether the last scrape was successful (1 = success, 0 = failure)

## Installation

### From Source

```bash
go install github.com/JetSquirrel/openclaw_expoter@latest
```

### Build Locally

```bash
git clone https://github.com/JetSquirrel/openclaw_expoter.git
cd openclaw_expoter
go build -o openclaw_exporter .
```

## Usage

### Command Line Flags

```bash
./openclaw_exporter [flags]
```

Available flags:
- `-openclaw.dir` - Path to openclaw data directory (can also be set via `OPENCLAW_DIR` environment variable)
- `-web.listen-address` - Address to listen on for web interface and telemetry (default: `:9101`)
- `-web.telemetry-path` - Path under which to expose metrics (default: `/metrics`)

### Examples

Using command line flag:
```bash
./openclaw_exporter -openclaw.dir=/path/to/openclaw/data
```

Using environment variable:
```bash
export OPENCLAW_DIR=/path/to/openclaw/data
./openclaw_exporter
```

Custom listen address:
```bash
./openclaw_exporter -openclaw.dir=/path/to/openclaw/data -web.listen-address=:9090
```

### Docker

Create a `Dockerfile`:

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o openclaw_exporter .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/openclaw_exporter /usr/local/bin/
EXPOSE 9101
ENTRYPOINT ["/usr/local/bin/openclaw_exporter"]
```

Build and run:
```bash
docker build -t openclaw_exporter .
docker run -d -p 9101:9101 -v /path/to/openclaw/data:/data openclaw_exporter -openclaw.dir=/data
```

## Prometheus Configuration

Add the following to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'openclaw'
    static_configs:
      - targets: ['localhost:9101']
    scrape_interval: 30s
```

## Metrics Endpoint

Once running, metrics are available at `http://localhost:9101/metrics`.

Example output:
```
# HELP openclaw_file_size_bytes Size of openclaw files in bytes
# TYPE openclaw_file_size_bytes gauge
openclaw_file_size_bytes{file="soul.md"} 1024
openclaw_file_size_bytes{file="skill.md"} 2048
openclaw_file_size_bytes{file="agent.md"} 512

# HELP openclaw_file_mtime_seconds Last modification time of openclaw files in seconds since epoch
# TYPE openclaw_file_mtime_seconds gauge
openclaw_file_mtime_seconds{file="soul.md"} 1707828000
openclaw_file_mtime_seconds{file="skill.md"} 1707828100
openclaw_file_mtime_seconds{file="agent.md"} 1707828200

# HELP openclaw_context_length_total Total length of context window in characters
# TYPE openclaw_context_length_total gauge
openclaw_context_length_total 15360

# HELP openclaw_skills_total Total number of skills
# TYPE openclaw_skills_total gauge
openclaw_skills_total 5

# HELP openclaw_agents_total Total number of agents
# TYPE openclaw_agents_total gauge
openclaw_agents_total 3

# HELP openclaw_scrape_success Whether the last scrape was successful
# TYPE openclaw_scrape_success gauge
openclaw_scrape_success 1
```

## Architecture

The exporter follows Prometheus best practices:

- **Collector Pattern**: Implements `prometheus.Collector` interface for efficient metric collection
- **Pull Model**: Prometheus scrapes metrics on-demand
- **Idiomatic Go**: Follows [Effective Go](https://go.dev/doc/effective_go) conventions
- **Error Handling**: Graceful error handling with scrape success indicator
- **Minimal Dependencies**: Only uses official Prometheus client library

## Development

### Project Structure

```
.
├── main.go              # HTTP server and entry point
├── collector/
│   └── collector.go     # Metrics collectors
├── go.mod
├── go.sum
└── README.md
```

### Building

```bash
go build -o openclaw_exporter .
```

### Testing

Create a test openclaw directory:
```bash
mkdir -p /tmp/openclaw_test
echo "# Soul" > /tmp/openclaw_test/soul.md
echo "# Skills\n## Skill 1\n## Skill 2" > /tmp/openclaw_test/skill.md
echo "# Agents\n## Agent 1" > /tmp/openclaw_test/agent.md
```

Run the exporter:
```bash
./openclaw_exporter -openclaw.dir=/tmp/openclaw_test
```

Test the endpoint:
```bash
curl http://localhost:9101/metrics
```

## License

MIT License

## References

- [Openclaw Overview](https://deepwiki.com/openclaw/openclaw/1-overview)
- [Openclaw Agent Execution Flow](https://deepwiki.com/openclaw/openclaw/5.1-agent-execution-flow)
- [Openclaw Context Management](https://deepwiki.com/openclaw/openclaw/5.5-context-overflow-and-auto-compaction)
- [Prometheus Documentation](https://prometheus.io/docs/)
- [Writing Exporters](https://prometheus.io/docs/instrumenting/writing_exporters/)
