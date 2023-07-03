# append to https://github.com/kneu-messenger-pigeon/github-workflows/blob/main/Dockerfile
# see https://github.com/kneu-messenger-pigeon/github-workflows/blob/main/.github/workflows/build.yaml#L20
ENV LISTEN=:8080

HEALTHCHECK --start-period=5s --interval=30s --timeout=3s \
  CMD wget --no-verbose --tries=1 --spider http://localhost${LISTEN}/healthcheck || exit 1
