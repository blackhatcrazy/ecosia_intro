FROM alpine:3.10.2 as alpine
RUN apk add -U --no-cache ca-certificates

FROM scratch

WORKDIR /
COPY --from=alpine /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY "./app" /

WORKDIR /
ENTRYPOINT ["/tree-spotter"]

# Expose port
EXPOSE 8090

