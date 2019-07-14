#!/usr/bin/env bash

gcloud functions deploy taskUpdateTimetable \
  --entry-point=UpdateTimetable \
  --runtime=go111 \
  --trigger-topic=update-timetable \
  --region=asia-northeast1 \
  --env-vars-file=vars-production.yaml \
  --timeout=300s \
  --memory=2048MB
