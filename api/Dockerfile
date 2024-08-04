# Start from golang base image
FROM golang:latest as base

# Set the working directory
WORKDIR /app

# Install Air for hot reloading in development
RUN go install github.com/air-verse/air@latest

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code into the container
COPY . .

# Build argument to determine the environment
ARG BUILD_ENV=production

# Development stage
FROM base as development
# Copy Air configuration if it exists, otherwise create a default one
COPY .air.toml* ./.air.toml
RUN if [ ! -f .air.toml ]; then \
    echo -e "root = \".\"\ntmp_dir = \"tmp\"\n\n[build]\n  cmd = \"go build -o ./tmp/main .\"\n  bin = \"./tmp/main\"\n  delay = 1000\n  exclude_dir = [\"assets\", \"tmp\", \"vendor\"]\n  include_ext = [\"go\", \"tpl\", \"tmpl\", \"html\"]\n  exclude_regex = [\"_test\\.go\"]\n\n[log]\n  time = false\n\n[color]\n  main = \"magenta\"\n  watcher = \"cyan\"\n  build = \"yellow\"\n  runner = \"green\"\n\n[misc]\n  clean_on_exit = true" > .air.toml; \
    fi
# Command to run Air for hot reloading
CMD ["air"]

# Production stage
FROM base as production
# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Create a minimal production image
FROM alpine:latest as production-final
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=production /app/main .
CMD ["./main"]

# Final stage - controlled by build argument
FROM development

# Expose port 8080 to the outside world
EXPOSE 8080