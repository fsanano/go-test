FROM golang:1.24-alpine

WORKDIR /app

# Install git and air
# git is often needed for go mod download if dependencies are private or complex
RUN apk add --no-cache git && \
    go install github.com/air-verse/air@v1.61.0

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Run air
CMD ["air"]
