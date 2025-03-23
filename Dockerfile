FROM    golang:1.24-bookworm AS build

ARG     BUILD_VERSION=latest
LABEL   stage=env-cleaner-${BUILD_VERSION}

WORKDIR /go/src/github.com/fragpit/env-cleaner/
COPY    . .

ENV     GOCACHE="/go/src/github.com/fragpit/env-cleaner/.cache"
RUN     --mount=type=cache,target="${GOCACHE}" \
        set -eux; \
          mkdir -p "${GOCACHE}" ;\
          echo "--- Build app ---" ;\
          make build

FROM    bookworm-slim

LABEL   name="env-cleaner"
ENV     DEBIAN_FRONTEND=noninteractive \
        TERM=xterm

RUN     set -eux; \
          echo "--- Install required packages ---" ;\
          apt update ;\
          apt install -y --no-install-recommends \
            curl \
            libc6 \
            ca-certificates \
          ;\
          echo "--- Create user for service ---" ;\
          groupadd -g 999 serviceuser ;\
          useradd -r -u 999 -g serviceuser serviceuser ;\
          \
          echo "--- cleanup ---" ;\
          apt clean ;\
          rm -rf /tmp/* ;\
          rm -rf /var/lib/apt/lists/* ;\
          rm -rf /root/.cache

COPY    --from=build /go/src/github.com/fragpit/env-cleaner/bin/env-cleaner /usr/local/bin

USER    serviceuser
