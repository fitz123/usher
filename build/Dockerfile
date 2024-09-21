# Build stage using a smaller Go image
FROM golang:1.23-alpine as builder
WORKDIR /app

# Install essential build tools
RUN apk add --no-cache make git

# Copy dependency information and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire project
COPY . .

# Build the application using the Makefile
RUN make build

# The builder stage ends here with the binary ready

