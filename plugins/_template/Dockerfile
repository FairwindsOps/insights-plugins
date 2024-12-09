# ***NOTE*** This file requires editing.                                        
  # Replace newplugin with the name of the new plugin.                            
#This Dockerfile is used with goreleaser, see also .goreleaser.yml.envsubst
# Goreleaser builds the binary, which is copied into the Docker image here.
FROM alpine:3.21
WORKDIR /usr/local/bin
RUN apk -U upgrade
RUN apk --no-cache add ca-certificates

USER 1200
COPY newplugin .

WORKDIR /opt/app

CMD ["newplugin"]
