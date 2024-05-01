FROM postgres:14.10-alpine3.19

USER root
ENV PG_PARTMAN_VERSION v4.6.2

RUN apk -U upgrade
RUN apk update && apk upgrade

RUN apk add curl make gcc libc-dev postgresql-dev

RUN curl -L https://github.com/pgpartman/pg_partman/archive/$PG_PARTMAN_VERSION.tar.gz -o "pg_partman.tar.gz"
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

USER 1001
