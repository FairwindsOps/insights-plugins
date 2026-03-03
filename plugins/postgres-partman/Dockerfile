# do not change cloudnative-pg base image as it is required for kubernetes deployment
FROM ghcr.io/cloudnative-pg/postgresql:17.7

USER root

# Apply security updates for OS packages (reduces Debian/base image vulns)
RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get upgrade -y \
    && rm -rf /var/lib/apt/lists/*

# Upgrade Python packages with known vulns (from base image; may reduce python-pkg findings)
RUN python3 -m pip install --no-cache-dir -U urllib3 cryptography protobuf pyasn1 azure-core 2>/dev/null || true

ENV PG_PARTMAN_VERSION=v5.1.0
RUN apt-get update && apt-get install -y \
    wget \
    make \
    gcc \
    libc6-dev \
    postgresql-server-dev-17
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
