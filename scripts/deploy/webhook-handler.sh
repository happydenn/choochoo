#!/usr/bin/env bash

gcloud functions deploy lineWebhook \
  --entry-point=HandleLINEWebhook \
  --runtime=go111 \
  --trigger-http \
  --region=asia-northeast1 \
  --env-vars-file=vars-production.yaml \
