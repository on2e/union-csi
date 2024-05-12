# Build gogomergerfs binary
ARG GO_VERSION=latest
FROM golang:${GO_VERSION} AS build-gogomergerfs
WORKDIR /src
COPY . /src
RUN make \
    && mv ./bin/gogomergerfs /usr/local/bin/gogomergerfs \
    && make clean

# Build mergerfs binary
FROM alpine:latest AS build-mergerfs
RUN apk update && apk upgrade \
    && apk add git
WORKDIR /src
ARG MERGERFS_VERSION
RUN git clone https://github.com/trapexit/mergerfs.git /src && git fetch && git checkout ${MERGERFS_VERSION} \
    && ./tools/install-build-pkgs \
    && make STATIC=1 \
    && strip --strip-all ./build/mergerfs \
    && mv ./build/mergerfs /usr/local/bin/mergerfs \
    && make distclean

# Build final image
# NOTE: go with alpine but revisit
FROM alpine:latest
COPY --from=build-gogomergerfs /usr/local/bin/gogomergerfs /usr/local/bin/gogomergerfs
COPY --from=build-mergerfs /usr/local/bin/mergerfs /usr/local/bin/mergerfs
ENTRYPOINT ["gogomergerfs"]
