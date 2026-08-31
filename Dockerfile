FROM golang:1.26.1-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

EXPOSE 8080

CMD [ "go", "run", "main.go" ]


