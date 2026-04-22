# do not change cloudnative-pg base image as it is required for kubernetes deployment
# Use minimal-trixie for fewer packages and newer Debian (fewer vulns than default bullseye)
FROM ghcr.io/cloudnative-pg/postgresql:18.3-minimal-trixie

USER root

# Apply security updates for OS packages
RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get upgrade -y \
    && rm -rf /var/lib/apt/lists/*

ENV PG_PARTMAN_VERSION=v5.4.3
# Build pg_partman then remove toolchain and headers: they pull linux-libc-dev / gcc
# (dozens of HIGH CVEs in scanners) and are not required to run PostgreSQL.
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        wget \
        ca-certificates \
        make \
        gcc \
        libc6-dev \
        postgresql-server-dev-18 \
    && wget -O /tmp/pg_partman.tar.gz "https://github.com/pgpartman/pg_partman/archive/refs/tags/$PG_PARTMAN_VERSION.tar.gz" \
    && mkdir -p /usr/src/pg_partman \
    && tar --extract --file /tmp/pg_partman.tar.gz --directory /usr/src/pg_partman --strip-components 1 \
    && rm /tmp/pg_partman.tar.gz \
    && cd /usr/src/pg_partman \
    && make \
    && make install \
    && rm -rf /usr/src/pg_partman \
    && apt-get purge -y --auto-remove \
        wget \
        make \
        gcc \
        libc6-dev \
        postgresql-server-dev-18 \
    && rm -rf /var/lib/apt/lists/*

USER 999
