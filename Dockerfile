# Build union-csi-driver binary
ARG GO_VERSION=latest
FROM golang:${GO_VERSION} AS build-union-csi-driver
WORKDIR /src
COPY . /src
RUN make \
    && mv ./bin/union-csi-driver /usr/local/bin/union-csi-driver \
    && make clean

# Build final image
# NOTE: go with alpine but revisit
FROM alpine:latest
COPY --from=build-union-csi-driver /usr/local/bin/union-csi-driver /usr/local/bin/union-csi-driver
ENTRYPOINT ["union-csi-driver"]
