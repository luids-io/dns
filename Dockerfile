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
RUN adduser -D -g 'luids' ludns

# Import the user and group files from the builder.
COPY --from=build-env /app/bin/ludns /bin/ludns
COPY --from=build-env /app/configs/docker/services.json /etc/luids/
COPY --from=build-env /app/configs/docker/ludns/* /etc/luids/dns/

# Set capabilities
RUN setcap CAP_NET_BIND_SERVICE=+eip /bin/ludns

USER ludns

ENV FORWARD_DNS 8.8.8.8:53

EXPOSE 53 53/udp

VOLUME [ "/etc/luids" ]

CMD [ "/bin/ludns", "-conf", "/etc/luids/dns/Corefile" ]
