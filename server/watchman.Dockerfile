FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    watchman \
    tzdata \
    && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /watchman-sock /data/storage /data/storage/primary

ENTRYPOINT ["watchman", "--foreground", "--sockname=/watchman-sock/watchman.sock"]
