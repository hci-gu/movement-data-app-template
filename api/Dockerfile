FROM alpine:latest

ARG GOLANG_VERSION=1.22.0

RUN apk update && apk add go gcc bash musl-dev openssl-dev ca-certificates && update-ca-certificates

RUN wget https://dl.google.com/go/go$GOLANG_VERSION.src.tar.gz && tar -C /usr/local -xzf go$GOLANG_VERSION.src.tar.gz
RUN cd /usr/local/go/src && ./make.bash
ENV PATH=$PATH:/usr/local/go/bin
RUN rm go$GOLANG_VERSION.src.tar.gz

#we delete the apk installed version to avoid conflict
RUN apk del go

# add go files
ADD main.go /pb/main.go
ADD go.mod /pb/go.mod
ADD go.sum /pb/go.sum
COPY ./migrations /pb/migrations

# build binary
RUN cd /pb && go build -o app

# uncomment to copy the local pb_hooks dir into the image
# COPY ./pb_hooks /pb/pb_hooks

EXPOSE 8080

# start PocketBase
CMD ["/pb/app", "serve", "--http=0.0.0.0:8080"]