#!/bin/bash

# Get env cli arg
env=$1

# if env == dev add a suffix
suffix=""
echo ${env}
if [[ "$env" == "dev" ]]; then
  echo "DEPLOYING TO DEV"
  suffix="-dev"
fi

# Loop through each folder
for folder in "build-s3" "build-git" "build-empty"; do
  echo "Building image for $folder"
  cd $folder
  docker buildx build --platform linux/amd64 -f Dockerfile.dev -t genezio-$folder$suffix .
  docker tag genezio-$folder$suffix harbor-registry.prod.cluster.genez.io/genezio/genezio-$folder$suffix
  docker push harbor-registry.prod.cluster.genez.io/genezio/genezio-$folder$suffix
  cd ..
done
