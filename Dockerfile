ARG GO_VERSION=${GO_VERSION:-1.19}

FROM golang:${GO_VERSION}-alpine AS builder

RUN apk update && apk add --no-cache git

WORKDIR /src

COPY ./go.mod ./go.sum ./
RUN go mod download

COPY . .

RUN cat /etc/passwd | grep nobody > /etc/passwd.nobody

# Build the binary.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -tags=nomsgpack -o /app .

# build a small image
FROM alpine

ENV TZ=Europe/Kyiv
ENV LISTEN=:8080
RUN apk add tzdata

COPY --from=builder /etc/passwd.nobody /etc/passwd
COPY --from=builder /app /app

# Run
USER nobody
ENTRYPOINT ["/app"]

HEALTHCHECK --start-period=5s --interval=30s --timeout=3s \
  CMD wget --no-verbose --tries=1 --spider http://localhost${LISTEN}/healthcheck || exit 1
