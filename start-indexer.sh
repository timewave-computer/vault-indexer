#!/bin/bash

# Default to dev environment if not specified
ENV=${1:-local}

# Validate environment
if [[ "$ENV" != "local" && "$ENV" != "prod" ]]; then
    echo "Error: Environment must be either 'local' or 'prod'"
    exit 1
fi

# Export environment variable
export ENV=$ENV

# Run the application
go run main.go 