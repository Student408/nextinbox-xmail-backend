# Use a lightweight Go image for building
FROM golang:1.23.3-alpine AS builder

# Install necessary tools in the builder image
RUN apk add --no-cache git

# Set the working directory inside the container
WORKDIR /app

# Copy Go module files first and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application files
COPY . .

# Build the application
RUN go build -o nextinbox

# Start a new stage with a minimal image
FROM alpine:latest

# Set the working directory
WORKDIR /app

# Copy the built binary and .env file from the builder stage
COPY --from=builder /app/nextinbox .
# COPY .env .env

# Expose the port your app listens on
EXPOSE 8080

# Command to run the application
CMD ["./nextinbox"]
