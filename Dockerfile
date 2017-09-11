FROM scratch

ARG DOCKER_BINARY

ADD $DOCKER_BINARY /service/bin/nsq-traefik-consumer
USER 65534:65534

ENTRYPOINT ["/service/bin/nsq-traefik-consumer"]
