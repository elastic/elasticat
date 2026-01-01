# Log Files Directory

Place your application log files here (or symlink to them) and TurboDevLog will automatically collect them.

## Supported Formats

### Plain Text Logs
Standard log formats with optional timestamp and level:
```
2024-01-15 10:30:45 INFO Starting server on port 8080
2024-01-15 10:30:46 ERROR Failed to connect to database
```

### JSON Logs
Structured JSON logs (one object per line):
```json
{"timestamp":"2024-01-15T10:30:45Z","level":"INFO","message":"Starting server","port":8080}
{"timestamp":"2024-01-15T10:30:46Z","level":"ERROR","message":"Connection failed","error":"timeout"}
```

## File Naming Convention

The service name is extracted from the filename:

| Filename | Service Name | Log Type |
|----------|--------------|----------|
| `server.log` | server | - |
| `server-err.log` | server | err |
| `api-error.log` | api | error |
| `myapp-debug.log` | myapp | debug |

## Usage Examples

### Option 1: Copy/Move Logs Here
```bash
cp /path/to/your/app/server.log ./logs/
```

### Option 2: Symlink Your Log Directory
```bash
# Symlink a single file
ln -s /var/log/myapp/server.log ./logs/server.log

# Or symlink an entire directory's contents
ln -s /var/log/myapp/*.log ./logs/
```

### Option 3: Point to a Different Directory
Set the `TURBODEVLOG_LOGS_DIR` environment variable:
```bash
TURBODEVLOG_LOGS_DIR=/var/log/myapp docker compose up -d
```

Or in your `.env` file:
```
TURBODEVLOG_LOGS_DIR=/var/log/myapp
```

## Log Flow

```
./logs/server.log
    │
    ▼
filelog/app receiver (polls every 500ms)
    │
    ▼
Parse: JSON or plain text auto-detected
    │
    ▼
Extract: service name, log level from filename/content
    │
    ▼
Elasticsearch (turbodevlog-logs index)
    │
    ▼
TUI viewer / Kibana / AI via MCP
```
