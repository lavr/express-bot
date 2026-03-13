FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION}" -o /express-bot .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=build /express-bot /usr/local/bin/express-bot
ENTRYPOINT ["express-bot"]
