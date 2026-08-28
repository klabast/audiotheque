FROM golang:1.27-alpine AS builder

WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN go build -o /test-mpd .

FROM alpine:3.24
COPY --from=builder /test-mpd /usr/local/bin/test-mpd
EXPOSE 6600 6601
CMD ["test-mpd"]
