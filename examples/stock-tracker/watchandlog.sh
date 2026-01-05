#!/bin/sh -ex
docker compose -f examples/stock-tracker/docker-compose.yml logs -f gateway stock-service portfolio-service 2>&1 &
../../bin/elasticat watch --service stock-tracker /dev/stdin
