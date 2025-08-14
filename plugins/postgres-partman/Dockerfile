# do not change cloudnative-pg base image as it is required for kubernetes deployment
FROM ghcr.io/cloudnative-pg/postgresql:17.5

USER root

ENV PG_PARTMAN_VERSION=v5.1.0
RUN apt-get update
RUN install_packages wget
RUN install_packages make
RUN install_packages gcc
RUN install_packages libc6-dev
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
