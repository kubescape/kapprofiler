#!/bin/bash

export NAMESPACE=ig-wl-filetracer-dev-env
export POD=$(kubectl -n $NAMESPACE get pods -l k8s-app=ig-wl-filetracer-dev-env -o jsonpath="{.items[0].metadata.name}")

kubectl exec -it $POD -n $NAMESPACE -- /bin/bash