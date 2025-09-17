# Etapa de build
FROM golang:1.24-rc-bullseye AS build

    WORKDIR /app

    COPY . .
    COPY cmd/api/main.go ./
    COPY go.mod go.sum ./

    RUN go mod download

    RUN CGO_ENABLED=0 go build -o /server

# Etapa de desenvolvimento
FROM build AS development

    RUN go install -v github.com/go-delve/delve/cmd/dlv@latest
    RUN go install github.com/cortesi/modd/cmd/modd@latest
    RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.55.2

    COPY .env ./

    CMD ["modd", "-f", "./cmd/api/modd.conf"]

# Etapa de produção
FROM alpine:latest AS production

    RUN apk update && apk add --no-cache ca-certificates

    COPY --from=build /server /server

    ENTRYPOINT ["/server"]
