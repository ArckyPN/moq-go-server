FROM ubuntu:latest

WORKDIR /build

# obligatory update
RUN apt-get update

# install depencies
RUN apt-get -y install gnupg iproute2 wget tar

# download go
RUN wget https://go.dev/dl/go1.22.2.linux-amd64.tar.gz

# unpack
RUN tar -C /usr/local -xzf go1.22.2.linux-amd64.tar.gz

# add go to PATH
ENV PATH="PATH=$PATH:/usr/local/go/bin"

COPY ./src .

EXPOSE 8080
EXPOSE 4433/udp

VOLUME [ "/cert", "/moq-client", "/data" ]

CMD go run . -tls_key /cert/localhost-key.pem -tls_cert /cert/localhost.pem -static /moq-client -data /data