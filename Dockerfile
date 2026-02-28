# syntax=docker/dockerfile:1

FROM alpine:latest AS builder
RUN mkdir -p /home/nonroot/.config /home/nonroot/.local/share

FROM gcr.io/distroless/static-debian13:nonroot
WORKDIR /home/nonroot
USER nonroot:nonroot

COPY --from=builder --chmod=700 --chown=nonroot:nonroot /home/nonroot/ .
VOLUME ["/home/nonroot/.config", "/home/nonroot/.local/share"]

ARG TARGETARCH
COPY --chmod=700 --chown=nonroot:nonroot dist/revchat_linux_${TARGETARCH}*/revchat .
ENTRYPOINT ["./revchat"]
