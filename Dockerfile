FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/cabinet-server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/cabinet-server /app/cabinet-server
COPY migrations /app/migrations
ENV MIGRATIONS_DIR=/app/migrations \
    HTTP_ADDR=:8080 \
    APP_ENV=production
EXPOSE 8080
USER nobody
ENTRYPOINT ["/app/cabinet-server"]
