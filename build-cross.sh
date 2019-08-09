#!/bin/bash

GOOS=linux GOARCH=amd64 go build -v  \
				-o build/bin/tug-linux-amd64 ./cmd/tug; \

scp build/bin/tug-linux-amd64 lijiang@10.222.111.4:/nfs_data/tugboat/.