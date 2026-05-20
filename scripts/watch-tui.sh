#!/bin/sh
set -e
go build -o bin/.watch/lazyagentcfg ./cmd/lazyagentcfg
exec bin/.watch/lazyagentcfg
