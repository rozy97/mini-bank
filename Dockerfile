FROM golang:1.26.1-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/mini-bank ./cmd

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/mini-bank .

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/app/mini-bank"]
