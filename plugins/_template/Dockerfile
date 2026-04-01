# ***NOTE*** This file requires editing.                                        
  # Replace newplugin with the name of the new plugin.                            
#This Dockerfile is used with goreleaser, see also .goreleaser.yml.envsubst
# Goreleaser builds the binary, which is copied into the Docker image here.
FROM alpine:3.23
WORKDIR /usr/local/bin
RUN apk --no-cache add ca-certificates \
    && apk --no-cache add --upgrade libcrypto3 libssl3 zlib

USER 1200
COPY newplugin .

WORKDIR /opt/app

CMD ["newplugin"]
