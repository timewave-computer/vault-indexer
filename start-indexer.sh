#!/bin/bash

# Default to dev environment if not specified
ENV=${1:-dev}

# Validate environment
if [[ "$ENV" != "dev" && "$ENV" != "prod" ]]; then
    echo "Error: Environment must be either 'dev' or 'prod'"
    exit 1
fi

# Export environment variable
export ENV=$ENV

# Run the application
go run main.go 