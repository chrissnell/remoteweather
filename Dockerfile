# GoReleaser will provide the binary
FROM alpine:3.19

# Install ca-certificates and create non-root user
RUN apk --no-cache add ca-certificates tzdata && \
    adduser -D -u 65532 -g remoteweather remoteweather && \
    mkdir -p /var/lib/remoteweather && \
    chown -R remoteweather:remoteweather /var/lib/remoteweather

WORKDIR /

# Copy the pre-built binary from GoReleaser
COPY remoteweather /usr/local/bin/remoteweather

# Switch to non-root user
USER remoteweather

# Data volume
VOLUME ["/var/lib/remoteweather"]

# Default port (adjust if needed)
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/remoteweather"]
CMD ["-config", "/var/lib/remoteweather/config.db"]