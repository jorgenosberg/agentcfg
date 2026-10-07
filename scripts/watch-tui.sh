#!/bin/sh
set -e
go build -o bin/.watch/agentcfg ./cmd/agentcfg
exec bin/.watch/agentcfg tui
