#!/bin/bash

# Run go run main.go for all example* directories

for dir in example*; do
    if [ -d "$dir" ]; then
        echo "Running: go run main.go $dir"
        go run main.go "$dir"
        echo "---"
    fi
done
