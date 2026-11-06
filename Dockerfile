FROM europe-docker.pkg.dev/northflank/public/harrier:1-none

USER root

RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    bash \
    vim \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN curl -sSL https://memdoor.ai/install.sh | sh

WORKDIR /harness
