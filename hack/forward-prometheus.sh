#!/usr/bin/env bash

set -xe

POD_NAME=$(kubectl get pods --namespace default -l "app.kubernetes.io/name=prometheus,app.kubernetes.io/instance=prometheus" -o jsonpath="{.items[0].metadata.name}")
exec kubectl --namespace default port-forward "$POD_NAME" 9090
