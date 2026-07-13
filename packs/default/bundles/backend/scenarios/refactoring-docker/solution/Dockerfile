# Build stage — compile the Go binary.
FROM golang:1.26 AS build

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app .

# Create non-root user for scratch image.
RUN useradd -M -s /bin/false -u 1000 app

# Runtime stage — empty base image.
FROM scratch

COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /app /app

USER app

EXPOSE 8080

ENTRYPOINT ["/app"]
