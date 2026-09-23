# Copyright 2026 [Copyright Holder]
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Author: [YOUR_NAME]

FROM alpine:latest
RUN apk add --no-cache ca-certificates curl tar tzdata

ARG TARGETARCH
RUN set -eux; \
    ARCH="${TARGETARCH:-}"; \
    if [ -z "$ARCH" ]; then \
        case "$(uname -m)" in \
            x86_64) ARCH="amd64" ;; \
            aarch64|arm64) ARCH="arm64" ;; \
            *) echo "Unsupported arch: $(uname -m)"; exit 1 ;; \
        esac; \
    fi; \
    curl -fsSL "https://github.com/sacloud/sacloud-otel-collector/releases/download/v0.8.0/sacloud-otel-collector_0.8.0_linux_${ARCH}.tar.gz" -o /tmp/collector.tar.gz && \
    tar -xzf /tmp/collector.tar.gz -C /usr/local/bin sacloud-otel-collector && \
    rm /tmp/collector.tar.gz && \
    chmod +x /usr/local/bin/sacloud-otel-collector

ENTRYPOINT ["/usr/local/bin/sacloud-otel-collector"]
CMD ["--config", "/etc/otelcol/config.yaml"]
