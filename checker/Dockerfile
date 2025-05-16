# syntax=docker/dockerfile:1
FROM golang:1.24-alpine

WORKDIR /app

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the binary
RUN go build -o premium-check

# Expose the API port
EXPOSE 8080

# Run the binary
CMD ["./premium-check"]
