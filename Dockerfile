# GoReleaser will provide the binary
FROM gcr.io/distroless/static-debian11:nonroot

WORKDIR /

# Copy the pre-built binary from GoReleaser
COPY remoteweather /remoteweather

# Use non-root user (uid 65532 in distroless)
USER nonroot:nonroot

# Data volume
VOLUME ["/var/lib/remoteweather"]

# Default port (adjust if needed)
EXPOSE 8080

ENTRYPOINT ["/remoteweather"]
CMD ["-config", "/var/lib/remoteweather/config.db"]