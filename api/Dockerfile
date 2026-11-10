FROM --platform=$BUILDPLATFORM golang:1.27.1-bookworm AS build
ARG TARGETOS
ARG TARGETARCH
ARG GO_BUILD_TAGS=
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -tags "$GO_BUILD_TAGS" -trimpath -o /out/app .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /pb/pb_data /pb/secrets \
    && chown 10001:0 /pb /pb/pb_data /pb/secrets \
    && chmod 0770 /pb /pb/pb_data && chmod 0750 /pb/secrets
WORKDIR /pb
COPY --from=build /out/app /pb/app
# Inject KontoAPI credentials and mount the evidence keyring at runtime.
USER 10001:0
EXPOSE 8080
CMD ["/pb/app", "serve", "--http=0.0.0.0:8080", "--dir=/pb/pb_data"]
