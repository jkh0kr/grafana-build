#!/usr/bin/env sh

go run ./cmd \
  package \
  publish \
  --distro=${DISTROS} \
  --build-id=${DRONE_BUILD_NUMBER} \
  --grafana-dir=${GRAFANA_DIR} \
  --github-token=${GITHUB_TOKEN} \
  --destination=${DESTINATION}/${DRONE_BUILD_EVENT} \
  --gcp-service-account-key-base64=${GCP_KEY_BASE64}
