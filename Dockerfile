FROM golang:1.25.5-alpine as BUILD
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
COPY internal/ ./internal/

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o validator .

# =======
FROM alpine:latest

RUN adduser -D appuser
WORKDIR /home/appuser

COPY --from=BUILD /app/validator .
USER appuser

EXPOSE 8080

CMD ["./validator"]