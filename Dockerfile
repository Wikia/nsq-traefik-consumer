FROM scratch

ARG DOCKER_BINARY

ADD $DOCKER_BINARY /service/bin/nsq-traefik-consumer

ENTRYPOINT ["/service/bin/nsq-traefik-consumer"]
