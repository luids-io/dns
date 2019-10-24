FROM golang:alpine as build-env
ARG arch=amd64

# Install git and certificates
RUN apk update && apk add --no-cache git make

WORKDIR /app
## dependences
COPY go.mod .
COPY go.sum .
RUN go mod download

## build
COPY . .
RUN make binaries SYSTEM="GOOS=linux GOARCH=${arch}"

## create docker
FROM alpine

LABEL maintainer="Luis Guillén Civera <luisguillenc@gmail.com>"

# Install git and certificates
RUN apk update && apk add --no-cache bind-tools libcap ca-certificates && update-ca-certificates

# create user for service
RUN adduser -D -g '' coredns

# Import the user and group files from the builder.
COPY --from=build-env /app/bin/ludns /bin/ludns
COPY --from=build-env /app/configs/docker/Corefile /etc/ludns/Corefile

# Set capabilities
RUN setcap CAP_NET_BIND_SERVICE=+eip /bin/ludns

USER coredns

ENV XLIST_ENDPOINT  tcp://xlist:5801
ENV RCACHE_ENDPOINT tcp://resolvcache:5891
ENV EVENT_ENDPOINT  tcp://event:5851

ENV FORWARD_DNS 8.8.8.8:53

EXPOSE 53 53/udp

VOLUME [ "/etc/ludns" ]

CMD [ "/bin/ludns", "-conf", "/etc/ludns/Corefile" ]
