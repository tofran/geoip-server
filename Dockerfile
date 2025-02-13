FROM golang:latest AS builder

WORKDIR /go/src/geoip-server

ADD ./ /go/src/geoip-server
RUN go mod vendor \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -o /geoip geoip.go

FROM alpine:latest
COPY --from=builder /geoip /geoip
CMD /geoip --edition=GeoLite2-Country --account-id=$MAXMIND_ACCOUNT_ID --license=$MAXMIND_LICENSE --allowed-origins=$ALLOWED_ORIGINS
