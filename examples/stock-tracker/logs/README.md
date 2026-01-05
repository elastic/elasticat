# Log Files

This directory is used to store Docker container logs for ElastiCat to watch.

## Usage

Capture logs from the demo services:

```bash
# From the stock-tracker directory
docker compose logs -f gateway stock-service portfolio-service > logs/demo.log 2>&1 &
```

Then use ElastiCat to watch and send to Elasticsearch:

```bash
# From the elasticat root directory
./bin/elasticat watch examples/stock-tracker/logs/demo.log
```

## Note

The `.gitkeep` file ensures this directory exists in the repository.
Log files (*.log) are ignored by git.


