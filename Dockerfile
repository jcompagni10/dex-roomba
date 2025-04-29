FROM golang:1.23-bullseye

WORKDIR /app


# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application with necessary flags
RUN GOFLAGS="-tags=netgo" CGO_ENABLED=1 go build -o main .

# Run the application
CMD ["./main"] 