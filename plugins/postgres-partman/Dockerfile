# do not change cloudnative-pg base image as it is required for kubernetes deployment
# Use minimal-trixie for fewer packages and newer Debian (fewer vulns than default bullseye)
FROM ghcr.io/cloudnative-pg/postgresql:18.3-minimal-trixie

USER root

# Apply security updates for OS packages
RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get upgrade -y \
    && rm -rf /var/lib/apt/lists/*

ENV PG_PARTMAN_VERSION=v5.4.2
RUN apt-get update && apt-get install -y \
    wget \
    make \
    gcc \
    libc6-dev \
    postgresql-server-dev-18
RUN wget -O pg_partman.tar.gz "https://github.com/pgpartman/pg_partman/archive/refs/tags/$PG_PARTMAN_VERSION.tar.gz"
RUN mkdir -p /usr/src/pg_partman \
    && tar \
        --extract \
        --file pg_partman.tar.gz \
        --directory /usr/src/pg_partman \
        --strip-components 1 \
    && rm pg_partman.tar.gz \
    && cd /usr/src/pg_partman \
    && make \
    && make install \
    && rm -rf /usr/src/pg_partman

USER 999
