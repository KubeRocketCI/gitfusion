# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
ARG TARGETARCH
WORKDIR /
COPY ./dist/api-${TARGETARCH} /api

USER 65532:65532

ENTRYPOINT ["/api"]
