# RECIPE HOW TO BUILD THIS PROJECT

FROM golang:1.26-alpine AS builder

# Working directory
WORKDIR /app

# Copy go.mod first to cache dependency downloads
# Add go.sum here once you have dependencies: COPY go.mod go.sum ./
COPY go.mod ./
RUN go mod download

# Copy rest of source code
COPY . .

# Build the binary — CGO_ENABLED=0 makes a static binary (no C dependencies included in the binary)
RUN CGO_ENABLED=0 GOOS=linux go build -o pinggoat ./cmd/api

# Stage 2: Run (tiny image, just the binary)
FROM alpine:3.21

WORKDIR /app

# Copy ONLY the built binary from stage 1
COPY --from=builder /app/pinggoat .

# Run the binary
CMD ["./pinggoat"]