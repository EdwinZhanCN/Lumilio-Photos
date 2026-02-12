FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    watchman \
    tzdata \
    && rm -rf /var/lib/apt/lists/*

ARG APP_UID=10001
ARG APP_GID=10001

RUN groupadd --gid ${APP_GID} app \
    && useradd --uid ${APP_UID} --gid ${APP_GID} --create-home --shell /usr/sbin/nologin app

RUN mkdir -p /watchman-sock /data/storage /data/storage/primary \
    && chown -R app:app /watchman-sock /data/storage /home/app

USER app

ENTRYPOINT ["watchman", "--foreground", "--sockname=/watchman-sock/watchman.sock"]
