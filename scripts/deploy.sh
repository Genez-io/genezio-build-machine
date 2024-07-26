set -e

# if $1 is empty then error out
if [ -z "$1" ]; then
    echo "Please provide an environment [dev/prod]"
    exit 1
fi

pwd 
docker buildx build --platform linux/amd64 -f Dockerfile.dev  --no-cache -t genezio-build-$1 .

docker tag genezio-build-$1 harbor-registry.prod.cluster.genez.io/genezio/genezio-build-$1

docker push harbor-registry.prod.cluster.genez.io/genezio/genezio-build-$1
