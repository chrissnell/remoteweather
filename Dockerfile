# --- Stage 1: Build the Go binary ---
        FROM golang:1.24-alpine AS builder

        # Install git (needed for fetching dependencies)
        RUN apk add --no-cache git
        
        # Set the working directory inside the container
        WORKDIR /app
        
        # Copy go.mod and go.sum first for dependency resolution
        COPY go.mod go.sum ./
        
        # Download dependencies
        RUN go mod download
        
        # Copy the rest of the application source code
        COPY . .
        
        # Ensure the build is for Linux x86_64 (since you're on macOS)
        RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o remoteweather .
        
        # --- Stage 2: Create a minimal runtime image ---
        FROM alpine:latest
        
        # Set working directory inside the container
        WORKDIR /app
        
        # Copy only the built binary from the builder stage
        COPY --from=builder /app/remoteweather .
        COPY entrypoint.sh .
        
        # Set the default command to run the application
        CMD ["./entrypoint.sh"]
        
